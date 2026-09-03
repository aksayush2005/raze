"""Gemini-backed advisory investigator.

Returns the SAME envelope as the heuristic advisor in service.py so callers and
the Go control plane are unchanged:

    {"recommendation", "confidence", "investigation": {...}, "model_version"}

This layer only recommends. The Go control plane applies everything, and every
error is raised so the dispatcher in service.py can fail closed or fall back to
heuristic-v1. The google-genai SDK is imported lazily so an uninstalled SDK or
an offline box never breaks the heuristic path.
"""

from __future__ import annotations

import json
import re
from typing import Any

# Recommendations the control plane understands (heuristic uses these too).
ALLOWED = ("RECOMMEND_MATCH", "REQUEST_REVIEW", "ESCALATE")
ALIASES = {
    "RECOMMEND_MATCH": "RECOMMEND_MATCH",
    "MATCH": "RECOMMEND_MATCH",
    "RECOMMENDED_MATCH": "RECOMMEND_MATCH",
    "AUTO_RECONCILE": "RECOMMEND_MATCH",
    "REQUEST_REVIEW": "REQUEST_REVIEW",
    "REVIEW": "REQUEST_REVIEW",
    "REQUIRES_REVIEW": "REQUEST_REVIEW",
    "ESCALATE": "ESCALATE",
    "ESCALATION": "ESCALATE",
}

MAX_SUMMARY = 2000

_SYSTEM = """\
You are the advisory investigation layer of RAZE, a financial reconciliation \
control plane. You examine ONE reconciliation case at a time.

Your inputs are the deterministic engine's output: the engine's decision \
(REVIEW or ESCALATE), its confidence, structured evidence (each with a type, a \
weight and details), candidate records (with matching strategy and \
similarity/score), and machine reasons.

Rules:
- Advise ONLY. You never mutate state, approve a payment, or change a ledger.
- Reason strictly from the supplied evidence and candidates. Never invent \
records, amounts, external ids, or facts not present in the input.
- Do NOT claim ground truth or certainty about a match the evidence does not \
support. The deterministic engine already routes REVIEW/ESCALATE because the \
arithmetic left something open — your job is to explain it and recommend a \
next step, not to second-guess integer arithmetic.
- Confidence must reflect how fully the supplied evidence explains the case; \
it is a decision signal, never financial truth.

Output STRICT JSON only, with exactly these keys:
{
  "recommendation": one of "RECOMMEND_MATCH" | "REQUEST_REVIEW" | "ESCALATE",
  "confidence": number between 0 and 1,
  "unexplained_evidence": [evidence type strings still unexplained],
  "summary": a concise plain-English explanation (<= 120 words) of the finding
}
No prose outside the JSON object."""


def _strip_fences(text: str) -> str:
    """Remove markdown code fences an SDK/backend might add around JSON."""
    text = text.strip()
    m = re.search(r"```(?:json)?\s*(.*?)```", text, re.DOTALL | re.IGNORECASE)
    if m:
        return m.group(1).strip()
    # Otherwise trim to the first balanced { ... }.
    start = text.find("{")
    end = text.rfind("}")
    if start != -1 and end > start:
        return text[start : end + 1]
    return text


def _normalise(raw: dict[str, Any]) -> dict[str, Any]:
    """Validate/normalise the LLM answer into the canonical envelope.

    Raises ValueError on anything that would corrupt the DB or the contract, so
    the dispatcher can fall back to the deterministic advisor.
    """
    rec = str(raw.get("recommendation", "")).strip().upper()
    if rec not in ALLOWED:
        rec = ALIASES.get(rec)
    if rec not in ALLOWED:
        raise ValueError(f"gemini returned unknown recommendation {raw.get('recommendation')!r}")

    try:
        conf = float(raw.get("confidence", 0.0))
    except (TypeError, ValueError):
        raise ValueError(f"gemini returned non-numeric confidence {raw.get('confidence')!r}")
    conf = max(0.0, min(conf, 0.99))  # mirror service.py clamp

    unexplained = raw.get("unexplained_evidence")
    if unexplained is None:
        unexplained = []
    elif not isinstance(unexplained, list):
        raise ValueError("gemini returned non-list unexplained_evidence")
    unexplained = [str(u) for u in unexplained][:20]

    summary = str(raw.get("summary", "")).strip() or "no summary supplied"
    summary = summary[:MAX_SUMMARY]

    return {
        "recommendation": rec,
        "confidence": round(conf, 4),
        "investigation": {
            "summary": summary,
            "unexplained_evidence": unexplained,
            "model": "gemini",  # human-friendly; model_version carries the id
        },
        "model_version": "",  # filled by the caller
    }


def advise(payload: dict[str, Any], *, api_key: str, model: str) -> dict[str, Any]:
    """Call Gemini and return a validated advisory envelope.

    Raises on any failure (network, SDK, schema) — the dispatcher decides
    whether that means fall back or fail closed.
    """
    from google import genai  # lazy: heuristic path must work without the SDK
    from google.genai import types

    client = genai.Client(api_key=api_key)
    case = {
        "item_id": payload.get("item_id"),
        "record_id": payload.get("record_id"),
        "engine_decision": payload.get("decision"),
        "engine_confidence": payload.get("confidence"),
        "candidates": payload.get("candidates", []),
        "evidence": payload.get("evidence", []),
        "reasons": payload.get("reasons", []),
        "engine_investigation": payload.get("investigation", {}),
    }
    resp = client.models.generate_content(
        model=model,
        contents="Investigate this reconciliation case and return the JSON:\n"
        + json.dumps(case, default=str),
        config=types.GenerateContentConfig(
            system_instruction=_SYSTEM,
            response_mime_type="application/json",
        ),
    )
    text = getattr(resp, "text", None)
    if not text:
        raise RuntimeError("gemini returned an empty response")

    raw = json.loads(_strip_fences(text))
    if not isinstance(raw, dict):
        raise ValueError("gemini returned non-object JSON")

    out = _normalise(raw)
    out["model_version"] = model
    return out
