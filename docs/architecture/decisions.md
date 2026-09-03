# RAZE Architecture Decisions

Status of each decision: **Accepted** unless marked *Proposed*.

## ADR-001 — Go owns authoritative financial state
- Go control plane is the *only* writer of authoritative financial records, state transitions, and audit entries.
- Python AI service is read-only over financial data and returns recommendations only. It never mutates authoritative records. Verified by API surface: Go applies every transition.

## ADR-002 — Integer minor units for money
- All monetary values are stored and computed as `int64` minor units (paise for INR, cents for USD).
- No floating point anywhere in financial truth. Deterministic integer arithmetic.

## ADR-003 — Go ↔ Python boundary uses HTTP/JSON (budget-justified deviation)
- The spec proposes gRPC + Protocol Buffers. To stay within the project's token budget and avoid protoc codegen tooling, the boundary is a small, versioned HTTP/JSON API.
- The interface (provider contract) is kept clean so gRPC can be added later without changing service logic.

## ADR-004 — DB-backed async jobs instead of Redis/Asynq (budget-justified deviation)
- Long-running investigations are modeled as DB-backed jobs with explicit workflow state, processed by in-process Go workers with bounded concurrency.
- Same state machine semantics as Redis/Asynq (PENDING → MATCHING → VERIFYING → INVESTIGATING → RESOLVED/ESCALATED), with durable job rows and optimistic locking. Redis remains optional infrastructure, not a correctness dependency.

## ADR-005 — AI service fails closed
- If the Python service is unavailable or errors, the control plane marks the item for human review — never auto-reconciles. A circuit breaker routes AI failures to REVIEW.

## ADR-006 — Razorpay Test Mode only
- Razorpay is behind a provider interface. Credentials come from env vars only; never hardcoded. When credentials are absent, the provider serves synthetic settlement data and every record is tagged `source=SYNTHETIC`. Test-mode settlement behavior is not claimed to match production.

## ADR-007 — Deterministic engine first, AI as ranker
- Candidate generation includes exact + deterministic fuzzy matching in the control plane. The Python layer ranks candidates, estimates confidence, and investigates ambiguity. AI confidence never overrides arithmetic inconsistency; deterministic validation always gates.

## ADR-008 — Versioned state transitions
- Workflow rows carry an optimistic-lock `version`. Writers use `UPDATE ... WHERE version = $n`; a stale write is rejected and surfaced (retry or conflict error). Every state change is recorded in the audit table.

## ADR-009 — Optional model-backed advisory investigator (Gemini)
- The advisory layer stays deterministic (`heuristic-v1`) so the demo never requires an LLM key or network.
- When `GEMINI_API_KEY` is set, each REVIEW/ESCALATE case is additionally advised by a Gemini model (`gemini-3.6-flash` by default — Google retired `gemini-2.5-flash` for new keys in Sept 2026; overridable via `GEMINI_MODEL`) behind the *same* `/advise` envelope — the Go client, `ai_decisions` schema, TUI and SPA are unchanged.
- `RAZE_AI_BACKEND` selects behaviour: `auto` (default — Gemini when keyed and reachable, silent fallback to heuristic on any LLM error), `gemini` (required — missing key or a failed call fails closed, so no advisory is stored), `heuristic` (always deterministic).
- Gemini output is validated/normalised in Python (recommendation forced into the 3-value enum, confidence clamped to `[0, 0.99]`, summary length-capped) and remains **advisory only**: the control plane persists it as an `ai_decisions` row and never auto-applies it. Credentials come from env only, never committed. Extends ADR-005 (fail closed) and ADR-007 (deterministic truth gates).
