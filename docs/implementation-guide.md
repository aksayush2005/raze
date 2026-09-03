# RAZE — Implementation Guide

> What was built, why it's shaped this way, and how it all fits together.
> Written for someone who wants to *learn* the system, not just run it.

---

## 1. What RAZE does

RAZE is an **autonomous financial reconciliation and risk-aware control** system.

**The problem:** A payment company (like Razorpay) moves money between many
sources — customers pay (`payment`), merchants get paid out (`settlement`),
fees and taxes are deducted. At the end of every day, someone has to check that
the internal ledger is *actually correct*: that every payment is accounted for,
that every settlement's amount matches the sum of its payments, that the
fees/taxes the provider charged match the arithmetic. Doing this by hand for
millions of transactions is slow and error-prone.

**The RAZE answer:** import both sides of the ledger into one normalized store,
then run a **deterministic reconciliation engine** over every record. The engine
produces a *confidence score* and routes each record to one of three buckets:

| Bucket      | Meaning                                                       |
|-------------|---------------------------------------------------------------|
| RECONCILED  | Everything checked out deterministically — safe to mark matched |
| REVIEW      | Plausible but not provable — a human must look at it           |
| ESCALATED   | Something is wrong or missing — escalate for investigation     |

An **AI service** (Python) is available to *recommend* what to do with REVIEW
and ESCALATED items — but it can only advise. It can never approve anything.

---

## 2. The non-negotiable rules (financial correctness)

These rules are the backbone of the design. Everything else bends around them.

1. **Never use floating point for money.** All amounts are **integer minor
   units** (paise for INR). `int64` in Go, `BIGINT` in Postgres. `100.00` is
   stored as `10000`.
2. **AI confidence is not financial truth.** A model saying "99.9% sure" never
   overrides arithmetic. The deterministic engine gates everything.
3. **High confidence never bypasses deterministic validation.** You can't get a
   high score unless *all* arithmetic checks passed.
4. **AI failures fail closed.** If the AI service is down or errors, items go to
   REVIEW/ESCALATE. Never the other way.
5. **Go owns authoritative financial state.** Python may generate candidates,
   rank them, investigate, and recommend — but it cannot write a single
   authoritative row. The Go control plane is the only writer.
6. **Critical decisions carry structured evidence.** Every reconciliation
   decision stores machine-verifiable evidence rows (e.g. `AMOUNT_EXPLAINED`
   with the exact sums), not just a number.
7. **Every state transition is audited.** Append-only audit trail: who did what,
   when, with what metadata.
8. **Human review happens through real backend APIs** — not a mock.
9. **Razorpay is TEST MODE only**, behind a provider interface, never hardcoded
   credentials, with a synthetic-data fallback so the whole thing demos without
   real money.

---

## 3. Architecture

```
                        ┌─────────────────────────────────────────────┐
                        │                 Go control plane            │
                        │                                             │
   import / sync ─────► │  handlers (HTTP API)                       │
                        │      │                                      │
                        │  services (orchestration: Reconciler,      │
                        │            ReviewService)                   │
                        │      │                                      │
                        │  engine (deterministic reconciliation)     │
                        │      │                                      │
                        │  repositories (the ONLY DB writer)         │
                        │      │                                      │
                        └──────┼──────────────────────────────────────┘
                               │
                       PostgreSQL ──► records, jobs, items, candidates,
                                     evidence, audit, reviews, AI
                               │
                        ┌──────┴──────────────────────────────────────┐
                        │  Python AI service (advisory only)           │
                        │  POST /advise → recommendation + confidence  │
                        └─────────────────────────────────────────────┘

   Human operator ──► Web UI (embedded SPA) ──► same HTTP API
   Razorpay TEST MODE ──► razorpay.Client ──► imported as records
```

Layers, bottom to top:

- **repositories** — the only thing that talks SQL. Every financial mutation
  goes through here. If you want to prove "Go owns state", this is the proof:
  the Python service has no database credentials.
- **engine** — pure deterministic logic. Takes one item, returns a decision,
  confidence, candidates, evidence, reasons. No side effects on its own.
- **services** — the orchestrator. Runs the async worker, drives state
  transitions, persists engine output + AI advice.
- **handlers** — HTTP. Parses/validates, calls services, writes JSON.
- **web** — embedded static SPA (served from the Go binary via `go:embed`).

---

## 4. Money: the `money` package

`internal/money/money.go` — `type Money int64`.

- `ParseDecimal("100.00")` → `Money(10000)`. Rejects more than 2 decimal
  places (`"1.005"` is an error) so nothing is silently truncated.
- `String()` → `"100.00"` for display.
- `Add` / `Sub` / `Neg` / `IsZero` — integer arithmetic, deterministic.

Every record column is `amount_minor`, `fee_minor`, `tax_minor`, `net_minor`
as `BIGINT`. The canonical relationship is `net = amount − fee − tax`.

> **Why it matters:** `0.1 + 0.2 == 0.3` is `false` in floating point. Money in
> floats is a source of off-by-one-paise bugs that are brutal to debug. Integers
> are exact, comparable, and summable — no rounding surprises.

---

## 5. Data model (the 9 tables)

`database/migrations/00001_init.sql` (applied by goose):

| Table                  | Purpose                                                     |
|------------------------|-------------------------------------------------------------|
| `records`              | Canonical normalized ledger rows (payment/settlement/…)      |
| `reconciliation_jobs`  | One run over all records; status + matched/review/escalated counters |
| `reconciliation_items` | One workflow unit per record in a job                        |
| `candidates`           | Proposed match relationships (exact ref / amount similarity) |
| `evidence`             | Structured, machine-verifiable support for a decision        |
| `audit_events`         | Immutable append-only trail of every state change            |
| `human_review_actions` | Operator decisions, recorded as structured feedback          |
| `ai_decisions`         | Advisory recommendations from Python (audited, never applied directly) |
| `idempotency_keys`     | Replay protection for state-changing requests                |

Two things to study closely in the schema:

**Optimistic locking.** `jobs` and `items` carry a `version` column. Every
update is `UPDATE ... WHERE id = $1 AND version = $2` and increments it. If two
writers race, the loser matches zero rows and gets `ErrStaleVersion` → the
caller retries or surfaces a conflict. This is how we protect against two
workers reconciling the same item.

**Idempotency keys.** A client sends `Idempotency-Key: <uuid>` on POSTs. The
server hashes the request body and stores the key → hash → response. On a
retry with the same key: same hash → replay the original response; different
hash → `409 Conflict`. This makes "did my import actually go through?" safe to
answer.

---

## 6. The reconciliation engine (the heart)

`internal/engine/engine.go`. Deterministic, integer-only, ~250 lines.

### How an item is processed

```
ProcessItem(item)
  └─ load the record
  └─ if settlement:
       find payments that reference it (ref_external_id = settlement id)
       if none → fuzzy match; else verify the batch
  └─ if payment:
       if it has ref_external_id → load that settlement, verify the batch
       if not → fuzzy match (amount-proximate opposite-kind record)
```

### The score

Start with a base of `0.10` and add evidence weights:

| Evidence                              | Weight | When                                    |
|---------------------------------------|--------|-----------------------------------------|
| `EXACT_REFERENCE_MATCH`               | +0.35  | payment ↔ settlement linked by ref      |
| `AMOUNT_SIMILARITY_MATCH`             | +0.20  | fuzzy match found (no explicit ref)     |
| `AMOUNT_EXPLAINED`                    | +0.25  | Σ(payments) == settlement amount        |
| `SETTLEMENT_ARITHMETIC_VALID`         | +0.25  | net == amount − fee − tax               |
| `AMOUNT_UNEXPLAINED`                  | −0.25  | gross doesn't add up                    |
| `ARITHMETIC_INCONSISTENT`             | −0.25  | declared net disagrees with arithmetic  |

Then the **risk policy** maps the score to a decision:

```
score ≥ 0.95  → RECONCILED
0.75 ≤ score  → REVIEW
score < 0.75  → ESCALATE
```

Worked example — a clean 1:1:
`0.10 base + 0.35 exact ref + 0.25 gross + 0.25 arithmetic = 0.95 → RECONCILED`.

A fuzzy match that fully verifies:
`0.10 + 0.20 amount-sim + 0.25 + 0.25 = 0.80 → REVIEW`.
Notice it **cannot** auto-reconcile: no explicit reference means a human confirms.

A gross mismatch:
`0.10 + 0.35 − 0.25 + 0.25 = 0.45 → ESCALATE`.

### Every corruption class is caught by exactly two checks

This is the design trick. Instead of a rule per corruption type, the engine
checks only two things:
1. **Gross consistency:** does Σ(payments) equal the settlement amount?
2. **Internal arithmetic:** does `net == amount − fee − tax`?

Because the generator produces `net` *before* corruption, any tampering with
amount, fee, or tax breaks at least one of these two checks — and a broken
check drops the score below 0.95, so it can never auto-reconcile.

| Corruption        | Which check catches it                          |
|-------------------|-------------------------------------------------|
| amount mismatch   | gross consistency (Σ payments ≠ settlement)     |
| fee discrepancy   | arithmetic (net ≠ amount − fee − tax)           |
| tax discrepancy   | arithmetic (net ≠ amount − fee − tax)           |
| missing settlement| the payment's ref resolves to nothing → MISSING_SETTLEMENT |
| reference corruption | ref missing → fuzzy → REVIEW; ref broken → ESCALATE |
| duplicate payment | Σ payments is inflated → gross mismatch         |
| date shift        | no impact on exact refs (a *soft* signal)       |

The engine also has an **ambiguity guard**: if more than one equally strong
match is found, it forces REVIEW and caps confidence at 0.70.

---

## 7. The async worker

`internal/services/reconcile.go`. A reconciliation runs over *all* records, so
it's background work — not a blocking HTTP request.

- **Claim semantics:** workers issue `SELECT ... FOR UPDATE SKIP LOCKED` —
  multiple worker goroutines can claim *different* items concurrently without
  stepping on each other. (If we used plain `SELECT`, two workers could grab the
  same item.)
- **Concurrency:** bounded by `WORKER_CONCURRENCY` (default 4).
- **Job lifecycle:** `PENDING → RUNNING → COMPLETED`. When no item is left in a
  non-terminal state, the job is marked COMPLETED.
- **Failure handling:** an item that errors goes to `FAILED` with an audit
  event. The job still completes.

Why DB-backed instead of Redis/Asynq? **Budget.** The spec proposed Redis/Asynq,
but a DB-backed worker gives the exact same semantics (claim, concurrency,
durability) using infrastructure we already have. Documented as ADR-004.

---

## 8. Human review

`internal/services/review.go`. When an item is in `REVIEW` or `ESCALATED`, an
operator (through the API or the UI) applies one of:

| Action                  | Effect                        |
|-------------------------|-------------------------------|
| `ACCEPTED_AGENT_MATCH`  | item → RESOLVED (matched)     |
| `REJECTED_CANDIDATE`    | item → ESCALATED              |
| `MANUALLY_LINKED_RECORDS` | item → RESOLVED with a specific target |
| `CONFIRMED_EXCEPTION`   | item → ESCALATED              |
| `ESCALATED`             | item → ESCALATED              |

Every action is one atomic transaction: insert the `human_review_actions` row,
optimistically update the item, append the audit event. It's real backend
state, recorded as structured feedback (which is exactly what a future ML
trainer would consume).

---

## 9. The AI advisory layer

`ai/` (FastAPI) + `internal/ai/client.go` (Go HTTP client).

- The Go worker calls `POST /advise` **only** for items that landed in
  REVIEW/ESCALATE.
- The Python service returns `{recommendation, confidence, investigation,
  model_version}` — **never a mutation**.
- The current implementation is a deterministic *heuristic* (so the demo needs
  no external LLM): it looks at the evidence and reasons, and:
  - no candidates → `ESCALATE` (confidence ≤ 0.30)
  - unexplained delta present → `REQUEST_REVIEW` (confidence capped at 0.70)
  - otherwise → `RECOMMEND_MATCH`
- The interface is shaped so a real model-backed investigator can replace the
  internals without touching any caller.
- **Fail closed:** if the service is down or errors, the control plane simply
  keeps the deterministic outcome (which already routes to REVIEW/ESCALATE).
- The recommendation is stored in `ai_decisions` for auditability — it's part
  of the record, but it never *is* the decision.

---

## 10. Razorpay (TEST MODE only)

`internal/razorpay/client.go`. Fetches settlements from Razorpay's test API.

- **Never instantiated** unless `RAZORPAY_MODE=test` **and** both credentials
  are set in env vars. No hardcoded keys anywhere; `.env.example` documents the
  shape.
- Mapped to records with `is_synthetic=false` and a clear `RZP_` external id
  prefix, so provider data is distinguishable from generated data.
- If not configured, the sync endpoint returns `503` with an honest message
  ("running on synthetic data") — the demo never requires real money.
- ADR-006 is explicit that test mode ≠ production behavior.

---

## 11. What's actually in each phase (files map)

| Phase | Deliverable | Where |
|-------|-------------|-------|
| 1. Config/architecture | env, Makefile, compose, ADRs | `.env.example`, `Makefile`, `docker-compose.yml`, `docs/architecture/decisions.md` |
| 2. Schema | 9 tables + indexes | `database/migrations/00001_init.sql` |
| 3. Go control plane | API + store + worker | `services/api/cmd/`, `internal/handlers`, `internal/repositories` |
| 4. Deterministic engine | scoring + risk policy | `services/api/internal/engine/engine.go` |
| 5. Synthetic data | generator + ground truth + evaluator | `data/generator/generate.py`, `evaluate.py` |
| 6. Razorpay test mode | provider client | `services/api/internal/razorpay/` |
| 7. Async workflow | DB worker, claim, job lifecycle | `services/api/internal/services/reconcile.go` |
| 8. Evidence/audit | evidence rows + append-only audit | `repositories/decisions.go`, migration tables |
| 9. Python AI | advisory FastAPI service | `ai/app/` |
| 10. Confidence/risk | policy thresholds, ambiguity guard | `engine.applyPolicy` |
| 11. Human review | real review API + structured actions | `services/review.go`, `handlers/items.go` |
| 12. Frontend | embedded workflow SPA | `services/api/internal/web/dist/` |
| 13. Integration test | full end-to-end proof | `services/api/api_integration_test.go` |
| 14. Docker/demo | images + one-command demo | `services/api/Dockerfile`, `ai/Dockerfile`, `scripts/demo.sh` |
| 15. Cleanup/docs | this guide + ADRs | `docs/implementation-guide.md`, `docs/architecture/decisions.md` |

---

## 12. How to run everything

```bash
# 1. Start Postgres
docker compose up -d postgres

# 2. Apply migrations
cd services/api && go run ./cmd/migrate

# 3. Generate synthetic data (with known ground truth)
python3 data/generator/generate.py --n-settlements 150 --corruption-rate 0.30 --seed 42

# 4. Start the control plane (+ AI service if you have fastapi/uvicorn)
cd services/api && go run ./cmd/api

# 5. Drive it from the web UI at http://localhost:8080
#    - Import the generated records.json
#    - "Run reconciliation"
#    - Open items to see evidence, AI investigation, and the human-review buttons
```

Or the whole thing automatically: `bash scripts/demo.sh`.

### The integration test (Phase 13) proves the system

`services/api/api_integration_test.go` boots the real app against real Postgres
and asserts the exact behavior I described:

- imports 10 records covering every decision band
- idempotency: same key replays, reused key with different body → 409
- job completes with `matched=2, review=2, escalated=6`
- a reconciled item carries `EXACT_REFERENCE_MATCH`, `AMOUNT_EXPLAINED`,
  `SETTLEMENT_ARITHMETIC_VALID` evidence and a match record
- an escalated item carries `AMOUNT_UNEXPLAINED` / `ARITHMETIC_INCONSISTENT`
- accepting a REVIEW item → `RESOLVED` + `human:<actor>` audit event

```bash
DATABASE_URL=postgres://raze:raze_dev_password@localhost:5432/raze?sslmode=disable \
  go test -run TestEndToEndReconciliation -v -count=1
```

---

## 13. Design tradeoffs worth learning from (the ADRs)

These are the decisions where I consciously deviated from the spec to stay
within budget, and documented *why*:

- **ADR-003:** Go ↔ Python over HTTP/JSON instead of gRPC/protobuf — avoids
  protoc codegen tooling. The provider interface stays clean so gRPC can be
  added later without touching service logic.
- **ADR-004:** DB-backed async jobs instead of Redis/Asynq — same semantics,
  less infrastructure.
- **Embedded SPA** instead of a Next.js app — the frontend is a tiny static
  page compiled into the Go binary (`go:embed`). No Node toolchain, no
  build pipeline, and the SPA talks to the same real API as any other client.

The general lesson: **the interface is the contract, the implementation is
replaceable.** As long as "Go owns authoritative state" and "AI only advises"
are enforced at the API boundary, you can swap HTTP↔gRPC, heuristic↔LLM,
in-process↔Redis without breaking the guarantees.

---

## 14. Honest verification status

> **Static verification: PASSED.** Every layer was re-read and cross-checked this
> session: the engine's scoring trace reproduces the integration test's exact
> expectations (2 matched / 2 review / 6 escalated across the 10-record set);
> every handler's request/response shape matches both the SPA and the test; the
> idempotency middleware restores the body and 409s on key reuse; human-review
> audit events are prefixed `human:<actor>`; the async worker's job lifecycle is
> wired to the `RUNNING` status it actually claims; and the AI client fails
> closed (nil when disabled, error when the service is down). One real issue was
> found and fixed during this pass: the `AIDecision` struct was not gofmt-aligned.
>
> Real bugs were caught and fixed during earlier review too (job-lifecycle
> deadlock, an undefined struct field, JSON tag mismatches between Go/Python/JS,
> nil-map inserts into NOT NULL columns, and a generator that was silently
> erasing corruption classes).
>
> **Dynamic verification: EXECUTED and PASSED** (after the environment was
> unblocked). Exact commands and results:
>
> ```bash
> cd services/api && go build ./...            # OK
> gofmt -l .                                   # clean (whole tree; six pre-existing files gofmt -w'd)
> go test -count=1 ./internal/...              # internal/money PASS (others: no test files)
> python3 -m py_compile data/generator/generate.py   # OK
> python3 data/generator/generate.py --n-settlements 5 --corruption-rate 0.5 --seed 7   # OK, all amounts int
> docker compose up -d postgres && cd services/api && go run ./cmd/migrate   # OK, version 1
> DATABASE_URL=postgres://raze:raze_dev_password@localhost:5432/raze?sslmode=disable \
>   go test -run TestEndToEndReconciliation -v -count=1   # PASS (0.40s)
> bash scripts/demo.sh                           # job COMPLETED: 402 records, 293 matched / 3 review / 106 escalated
> ```
>
> The integration test now reproduces exactly the intended outcomes:
> `matched = 2`, `review = 2`, `escalated = 6`, `total_records = 10`, RESOLVED
> items = 2 with `EXACT_REFERENCE_MATCH` / `AMOUNT_EXPLAINED` /
> `SETTLEMENT_ARITHMETIC_VALID` evidence, human review resolves a REVIEW item
> with a `human:<actor>` audit event, and the gross-mismatch / arithmetic
> mismatch escalations carry `AMOUNT_UNEXPLAINED` / `ARITHMETIC_INCONSISTENT`.
>
> **Real defects found and fixed during dynamic verification:**
>
> 1. **Fuzzy-candidate SQL could never resolve under pgx.** The query
>    `amount_minor BETWEEN $3 - $4 AND $3 + $4` and
>    `occurred_at BETWEEN $5 - make_interval(hours => $6) AND ...` left the
>    parameter types `unknown`, so Postgres failed with
>    `operator is not unique: unknown - unknown` (SQLSTATE 42725) at prepare
>    time. pgx only auto-retries with explicit type OIDs on 42P18, not 42725 —
>    so *every* no-reference record failed closed with `NO_CANDIDATE_FOUND` and
>    escalated. Fix: type-anchor the operands
>    (`$3::bigint - $4::bigint`, `$5::timestamptz - make_interval(hours => $6::int)`).
>    This is a general pgx/Postgres lesson: with the extended protocol, infer
>    parameter types from context; ambiguous arithmetic needs explicit casts.
> 2. **Idempotency keys never recovered after expiry.** `SetIdempotency`'s
>    `ON CONFLICT ... DO UPDATE` refreshed `response` but not `expires_at`, so a
>    key whose window passed stayed permanently expired: every retry re-ran the
>    handler and the "reuse with a different payload → 409" guard silently died.
>    Fix: refresh `expires_at = now() + interval '24 hours'` on reclaim.
> 3. **Typed-nil AI client crashed the worker.** `main.go` passed
>    `ai.NewClient("")` (a typed `*ai.Client` nil) into the `Investigator`
>    interface. An interface holding a nil pointer is non-nil, so the
>    reconciler's `r.ai != nil` guard passed and `Advise` was called on the nil
>    client → nil-pointer dereference → panic killed the whole API the moment
>    any REVIEW/ESCALATE item appeared with AI disabled. Fixes: build a nil
>    *interface* in `main.go` when the AI URL is empty, and guard `Advise`
>    against a nil receiver so a disabled client fails closed instead of
>    crashing.
> 4. **`scripts/demo.sh` never got past step 4.** `(cd services/api && go run
>    ./cmd/api &)` backgrounds the `go run` inside a subshell, so the parent's
>    `$!` was unbound and `set -u` aborted the script — and the EXIT trap could
>    never kill the API it never captured. Fix: build once
>    (`go build -o /tmp/raze-api ./cmd/api`), run the binary in the parent,
>    capture `$!`, and background the AI service the same way with `exec`.
>
> **One known gap (documented, not changed — kept conservative):** `ClaimItem`
> reclaims only `PENDING` items, while `CountActiveItems` treats
> `MATCHING/VERIFYING/INVESTIGATING` as active. If a worker dies after claiming
> (status → `MATCHING`), that item is never re-claimed and the job stays
> `RUNNING` forever. It only bites on a crash (the crash source above is fixed),
> so it was not touched this pass. Recommended follow-up: a stale-claim timeout
> in `ClaimItem`, e.g. `status IN ('PENDING','MATCHING','VERIFYING','INVESTIGATING')
> AND (status = 'PENDING' OR updated_at < now() - interval '10 minutes')`.

---

## Python AI service — heuristic-v1 vs Gemini (optional)

The investigation service in `ai/app/` answers `POST /advise` for REVIEW/ESCALATE
items and *recommends* `RECOMMEND_MATCH | REQUEST_REVIEW | ESCALATE` with a
confidence and an explanation. It never writes state — the Go reconciler only
persists its answer as an `ai_decisions` row (`services/api/internal/services/reconcile.go`),
which the web UI and TUI show under "AI investigation · advisory".

**Two interchangeable backends behind one envelope:**

- **`heuristic-v1`** (default, deterministic) — pure rules in
  `ai/app/service.py::_heuristic`: it examines evidence types/reasons for an
  unexplained delta, caps confidence below the auto threshold when one exists,
  and recommends accordingly. Needs no LLM, no key, no network.
- **Gemini** (optional) — `ai/app/gemini.py` sends the same case to a Gemini
  model (`gemini-3.6-flash` by default — Google retired `gemini-2.5-flash` for
  new API keys in Sept 2026 and the live API serves 3.6-flash). A system prompt
  constrains it to reason
  only from the supplied evidence, never invent facts, and return strict JSON.
  The answer is validated/normalised in Python (recommendation forced to the
  enum, confidence clamped to `[0, 0.99]`, summary length-capped) so nothing
  malformed reaches the database. `model_version` records which model advised.

**Routing (`RAZE_AI_BACKEND`, read from env in `service.py::advise`):**

| Mode | Behaviour |
|------|-----------|
| `auto` (default) | Gemini when `GEMINI_API_KEY` is set *and* the call succeeds; silently falls back to `heuristic-v1` if the key is missing or the LLM errors — reconciliation never blocks on the network |
| `gemini` | Gemini always; no key or a failed call fails closed (`/advise` → 500, no decision stored) |
| `heuristic` | Always the deterministic rules |

Set `GEMINI_API_KEY`, `GEMINI_MODEL`, `RAZE_AI_BACKEND` in the gitignored
`.env`. The AI service itself runs in `ai/.venv` — `scripts/tui.sh` (step 4/7)
creates it, installs `ai/requirements.txt` (which now includes `google-genai`),
and starts uvicorn on `:8090`; the API logs `ai=true` when it can see the
service. See ADR-009.

---

## Terminal UI (`raze-tui`)

The web SPA (embedded in the API binary) is not the only way to operate the
control plane. `services/api/cmd/raze-tui` is a **dependency-free Go terminal
client** that talks to the API over HTTP — the same endpoints the SPA uses —
with a keyboard-driven, workflow-first interface:

- **Jobs** — table of reconciliation runs with color-coded status pills; create
  a run (`n`), import `data/benchmark/records.json` (`i`), jump to records (`v`).
- **Job detail** — stat bar (records / matched / review / escalated) + item
  table; cycle the status filter (`f`); open an item with Enter.
- **Item workspace** — record, amount (fee/tax/net), matched record, confidence,
  decision, candidates, evidence with weights, AI investigation (*advisory*),
  audit trail; when the item is `REVIEW`/`ESCALATED`, apply a human review:
  `1` accept · `2` reject · `3` escalate · `4` confirm exception · `5` manual link.
- **Records** — normalized records table.

On launch the TUI plays a short **ASCII RAZE splash** — each letter of the
wordmark snaps in left→right (ghost outline → white pop → accent colour), an
underline draws itself, and the tagline types out underneath. Any key skips
straight into the jobs view; on terminals too small to fit it the splash is
skipped entirely.

Everything is read live from the API and refreshes on a ~2s ticker, so a
running job's progress is visible as it is consumed by the workers.

### Run it

```bash
make tui            # boots postgres + migrations + data + API, then the TUI
# or, against an API you already started:
cd services/api && go run ./cmd/raze-tui --api http://localhost:8080
```

Modes:

```bash
raze-tui --once                   # one plain-text snapshot, exit 0 (pipes cleanly)
raze-tui --watch --view job=3     # live non-interactive dashboard
raze-tui --view records           # start on the records view
raze-tui --actor operator@ops     # identity stamped on review actions
```

### Design notes

- **No new dependencies.** Raw terminal mode is handled by shelling out to
  `stty` (coreutils); input decoding and rendering are stdlib-only. This keeps
  the build working offline and honours the AGENTS.md dependency rule.
- **Money stays integer.** Display uses integer paise with Indian grouping
  (`formatMoney`), mirroring the control plane's "no floats for money" rule.
- **Pure client.** The TUI never reads the database and never mutates state
  except through the API's idempotent endpoints (`Idempotency-Key: tui-<nano>`).
  A down API shows a red error banner — it never fabricates data.
- **Two modes.** An interactive alt-screen UI, plus `--once`/`--watch` plain-text
  dumps that share the same renderers (colors are disabled when stdout is not a
  TTY so output stays greppable).
