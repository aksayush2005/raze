"""Advisory investigation logic.

This layer produces RECOMMENDATIONS only. It never mutates authoritative
financial state (the Go control plane owns that). Confidence here is a
decision signal, never financial truth.

Backends
--------
* heuristic-v1 (default, deterministic): fully demoable without an external
  LLM or network.
* Gemini (optional, advisory): when GEMINI_API_KEY is set, each case is also
  investigated by a model-backed advisor. The response envelope is identical.

Routing (RAZE_AI_BACKEND)
-------------------------
* auto (default): Gemini when GEMINI_API_KEY is set AND the call succeeds;
  heuristic-v1 when the key is absent or the call errors (never blocks on the
  LLM).
* gemini: always Gemini; missing key or a failed call raises -> /advise 500
  (fail closed, no AI decision stored).
* heuristic: always the deterministic rules.

The interface is shaped so a model-backed investigator can replace the
internals without changing callers.
"""

from __future__ import annotations

import logging
import os
from typing import Any

from . import gemini

MODEL_VERSION = "heuristic-v1"
# Google retired gemini-2.5-flash for new API keys (Sept 2026); 3.6-flash is
# what the live API serves and recommends.
DEFAULT_GEMINI_MODEL = "gemini-3.6-flash"

# Thresholds mirror the control-plane risk policy.
AUTO_THRESHOLD = 0.95
REVIEW_THRESHOLD = 0.75


def advise(payload: dict[str, Any]) -> dict[str, Any]:
    """Advisory investigation for one case, dispatched by RAZE_AI_BACKEND."""
    mode = os.getenv("RAZE_AI_BACKEND", "auto").strip().lower() or "auto"
    api_key = os.getenv("GEMINI_API_KEY", "").strip()
    model = os.getenv("GEMINI_MODEL", "").strip() or DEFAULT_GEMINI_MODEL

    if mode == "heuristic":
        return _heuristic(payload)

    if api_key:
        try:
            return gemini.advise(payload, api_key=api_key, model=model)
        except Exception as exc:  # noqa: BLE001 - deliberate policy branch
            if mode == "gemini":
                logging.error("gemini advise failed (fail-closed): %s", exc)
                raise
            logging.warning("gemini unavailable (%s); falling back to heuristic-v1", exc)

    if mode == "gemini":
        raise RuntimeError(
            "RAZE_AI_BACKEND=gemini but GEMINI_API_KEY is not set; no advisory produced"
        )

    return _heuristic(payload)


def _heuristic(payload: dict[str, Any]) -> dict[str, Any]:
    decision = payload.get("decision", "REVIEW")
    confidence = float(payload.get("confidence", 0.0))
    evidence: list[dict[str, Any]] = payload.get("evidence", [])
    candidates: list[dict[str, Any]] = payload.get("candidates", [])
    reasons: list[str] = payload.get("reasons", [])

    unexplained = [e for e in evidence
                   if "UNEXPLAINED" in e.get("type", "") or "INCONSISTENT" in e.get("type", "")]
    unexplained_evidence = [e["type"] for e in unexplained]
    has_unexplained = bool(unexplained) or any("DELTA" in r or "MISSING" in r for r in reasons)

    notes: list[str] = []
    if candidates:
        notes.append(f"examined {len(candidates)} candidate record(s)")
    if unexplained_evidence:
        notes.append("deterministic arithmetic leaves an unexplained delta: " + ", ".join(unexplained_evidence))
    if not has_unexplained and decision == "REVIEW":
        notes.append("no unexplained delta found; deterministic checks are consistent")

    # Confidence is adjusted only from evidence, never from free text.
    adjusted = confidence
    if has_unexplained:
        adjusted = min(adjusted, REVIEW_THRESHOLD - 0.05)  # cap below auto
    if not candidates:
        adjusted = min(adjusted, 0.30)

    # Recommendation selection (advisory).
    if not candidates:
        recommendation = "ESCALATE"
    elif has_unexplained or adjusted < AUTO_THRESHOLD:
        recommendation = "REQUEST_REVIEW"
    else:
        recommendation = "RECOMMEND_MATCH"

    adjusted = max(0.0, min(adjusted, 0.99))

    return {
        "recommendation": recommendation,
        "confidence": round(adjusted, 4),
        "investigation": {
            "summary": " ".join(notes) if notes else "no candidate evidence supplied",
            "unexplained_evidence": unexplained_evidence,
            "input_decision": decision,
            "input_confidence": confidence,
        },
        "model_version": MODEL_VERSION,
    }
