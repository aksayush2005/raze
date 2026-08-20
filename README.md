# RAZE

### Autonomous Financial Reconciliation & Risk-Aware Control

> **Raze reconciliation backlogs. Resolve financial discrepancies. Automate with evidence.**

RAZE is an agentic financial control system designed to reconcile heterogeneous payment and settlement records, investigate discrepancies, quantify decision confidence, and determine when financial operations can be safely automated or require human intervention.

The system combines:

* Deterministic financial reconciliation
* Machine-learning-based candidate matching
* Agentic investigation
* Structured evidence graphs
* Confidence-aware decision policies
* Human-in-the-loop feedback
* Risk-sensitive reinforcement learning
* Multi-currency and partial-settlement handling
* Asynchronous execution
* Production-oriented Go control infrastructure
* Reproducible financial benchmarks
* Stress and resilience testing

RAZE is designed for the **Razorpay AI Finance Controller** track and operates using **Razorpay Test Mode and controlled synthetic financial data**.

---

# 1. Problem

Financial operations are distributed across payments, orders, settlements, fees, taxes, refunds, chargebacks, and merchant ledgers.

Reconciliation therefore becomes a verification problem involving:

* Heterogeneous data sources
* Missing records
* Duplicate records
* Amount discrepancies
* Partial settlements
* Batched settlements
* Refunds
* Chargebacks
* Reference inconsistencies
* Currency conversion differences

Traditional rule-based systems handle straightforward cases well but struggle with ambiguous cases.

Large language models can reason about ambiguity but should not be trusted as the authoritative source of financial truth.

RAZE combines both approaches.

> **Deterministic systems establish financial correctness. AI investigates ambiguity. Humans retain control over uncertain cases.**

---

# 2. Core Principle

RAZE is designed around:

> **Trustworthy autonomy over maximum automation.**

The system separates the financial control layer from probabilistic intelligence.

```text
                     Financial Data
                           │
                           ▼
                 Deterministic Validation
                           │
                           ▼
                  Candidate Generation
                           │
                           ▼
                   Evidence Retrieval
                           │
                           ▼
                  Agentic Investigation
                           │
                           ▼
                 Confidence Estimation
                           │
                           ▼
                  Risk-Aware Policy
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
           MATCH        REVIEW       ESCALATE
              │            │            │
              └────────────┼────────────┘
                           ▼
                     Human Feedback
                           │
                           ▼
                  Learning / Calibration
                           │
                           ▼
                       Audit Trail
```

---

# 3. Research Questions

### RQ1 — Reconciliation

Can layered deterministic, fuzzy, and agentic reconciliation improve matching performance across heterogeneous financial records?

### RQ2 — Uncertainty

Can confidence-aware abstention reduce false autonomous reconciliations?

### RQ3 — Explainability

Can structured evidence graphs provide auditable explanations for autonomous financial decisions?

### RQ4 — Human Feedback

Can human resolution data improve confidence calibration and decision-policy performance?

### RQ5 — Risk-Sensitive Policy

Can reinforcement learning improve the automation–risk trade-off compared with static confidence thresholds?

### RQ6 — Scalability

How does deterministic Go-based reconciliation throughput compare with the latency and throughput of the AI investigation pipeline?

---

# 4. System Architecture

```text
                         ┌─────────────────────────┐
                         │       Next.js UI        │
                         │ Finance Operations      │
                         └────────────┬────────────┘
                                      │
                              REST / WebSocket
                                      │
                         ┌────────────▼────────────┐
                         │      Go Control Plane   │
                         │                         │
                         │ • API                    │
                         │ • Workflow State         │
                         │ • Financial Mutations    │
                         │ • Authorization          │
                         │ • Audit                  │
                         │ • Concurrency Control   │
                         │ • Circuit Breaker        │
                         └───────┬──────────┬──────┘
                                 │          │
                              gRPC       PostgreSQL
                                 │
                         ┌───────▼────────────┐
                         │ Python AI/ML Layer │
                         │                    │
                         │ • Matching         │
                         │ • Retrieval        │
                         │ • Agents           │
                         │ • XAI              │
                         │ • Confidence       │
                         │ • RL               │
                         │ • Evaluation       │
                         └────────┬───────────┘
                                  │
                         ┌────────▼─────────┐
                         │     pgvector     │
                         └──────────────────┘

               ┌────────────────────────────────────┐
               │       Financial Data Sources       │
               │                                    │
               │ Razorpay Test Mode + Synthetic     │
               │ Benchmark + Human Feedback         │
               └────────────────────────────────────┘
```

---

# 5. Architectural Boundaries

## Go Control Plane

Go is responsible for authoritative financial state.

Responsibilities:

* HTTP API
* Request validation
* Workflow orchestration
* Database transactions
* State transitions
* Financial mutations
* Authorization
* Audit logging
* Concurrency control
* Idempotency
* Async job submission
* Circuit breaking
* WebSocket event delivery

## Python AI/ML Layer

Python is responsible for probabilistic and research components.

Responsibilities:

* Candidate scoring
* Fuzzy matching
* Semantic retrieval
* Agentic investigation
* Evidence generation
* Confidence estimation
* RL inference
* RL training
* Benchmark generation
* Evaluation
* Calibration

The Python service cannot directly mutate authoritative financial state.

---

# 6. Financial Reconciliation Pipeline

```text
                 Raw Financial Records
                          │
                          ▼
                  Schema Normalization
                          │
                          ▼
                 Deterministic Checks
                          │
                          ▼
                  Candidate Generation
                          │
             ┌────────────┼────────────┐
             ▼            ▼            ▼
          Exact         Fuzzy       Semantic
          Match         Match        Match
             │            │            │
             └────────────┼────────────┘
                          ▼
                   Candidate Ranking
                          │
                          ▼
                  Evidence Verification
                          │
                          ▼
                  Agent Investigation
                          │
                          ▼
                  Confidence Estimation
                          │
                          ▼
                   Decision Policy
                          │
             ┌────────────┼────────────┐
             ▼            ▼            ▼
           Match        Review       Escalate
```

---

# 7. Deterministic Financial Verification

Core financial calculations are deterministic.

Example:

```text
Payment                  ₹10,000
Processing Fee              ₹150
Tax                          ₹27
────────────────────────────────
Expected Settlement       ₹9,823

Actual Settlement         ₹9,823
Difference                    ₹0
```

Result:

```text
RECONCILED
```

The system does not allow an LLM to override arithmetic inconsistencies.

---

# 8. Multi-Record Reconciliation

RAZE does not assume a simple 1-to-1 relationship.

It supports:

```text
1 Payment → 1 Settlement

N Payments → 1 Settlement

1 Payment → N Settlement Components

N Payments → N Settlement Components
```

This is important for realistic settlement workflows.

Example:

```text
PAY_001   ₹5,000
PAY_002   ₹8,000
PAY_003   ₹7,000
             │
             ▼
Settlement Batch
             │
        Gross ₹20,000
        Fee    ₹400
        Tax     ₹72
             │
             ▼
Net     ₹19,528
```

The reconciliation engine must determine whether the aggregate settlement can be explained by the underlying payment records.

---

# 9. Edge-Case Engine

The synthetic benchmark explicitly generates difficult financial scenarios.

### Amount Errors

```text
₹10,000 → ₹9,800
```

### Missing Records

Payment exists without settlement.

### Duplicate Records

The same payment appears multiple times.

### Partial Settlements

```text
Payment = ₹20,000
Settlement 1 = ₹12,000
Settlement 2 = ₹8,000
```

### Batched Settlements

Multiple payments contribute to one settlement.

### Refunds

A previously reconciled payment is partially refunded.

### Chargebacks

A settled transaction later produces a financial adjustment.

### Multi-Currency

```text
Authorization:
USD

Settlement:
INR

FX Rate:
time-dependent
```

The benchmark tracks:

* Original currency
* Settlement currency
* Exchange rate
* Converted amount
* FX difference

---

# 10. Agentic Investigation

RAZE uses specialized agents rather than an unrestricted autonomous agent.

```text
                    Controller
                        │
          ┌─────────────┼─────────────┐
          ▼             ▼             ▼
       Matcher       Verifier     Investigator
          │             │             │
          └─────────────┼─────────────┘
                        ▼
                 Decision Policy
                        │
              ┌─────────┴─────────┐
              ▼                   ▼
           Resolve             Escalate
```

### Controller

Coordinates the investigation.

### Matcher

Retrieves candidate financial records.

### Verifier

Checks numerical and structural consistency.

### Investigator

Searches available evidence to explain discrepancies.

### Decision Policy

Selects:

```text
AUTO_RECONCILE
REQUEST_REVIEW
INVESTIGATE
ESCALATE
```

The Go control plane authorizes any state-changing operation.

---

# 11. Explainable AI

RAZE uses **structured evidence objects** rather than free-form explanations.

Each investigation produces a typed decision object.

Example:

```json
{
  "reconciliation_id": "REC_9021",
  "decision": "RECOMMEND_MATCH",
  "confidence_score": 0.92,
  "evidence_chain": [
    {
      "type": "EXACT_REFERENCE_MATCH",
      "source": "PAY_10023",
      "target": "SETTLE_8801",
      "weight": 0.4
    },
    {
      "type": "FEE_BREAKDOWN_EXPLAINED",
      "breakdown": {
        "base": 12000,
        "fee": 200,
        "gst": 36
      },
      "unexplained_delta": 0,
      "weight": 0.5
    },
    {
      "type": "HISTORICAL_MERCHANT_PATTERN",
      "weight": 0.1
    }
  ],
  "abstention_reasons": []
}
```

The evidence object is machine-readable and directly rendered by the frontend.

---

# 12. Evidence Graph

The evidence chain is represented as a directed graph.

```text
PAY_10023
    │
    │ EXACT_REFERENCE_MATCH
    ▼
SETTLE_8801
    │
    ├── FEE_RECORD
    │       │
    │       ▼
    │     ₹200
    │
    └── TAX_RECORD
            │
            ▼
           ₹36

                ↓

          Financial Match
```

The graph allows operators to inspect:

* Which records contributed to the decision
* Which relationships were exact
* Which were inferred
* Which calculations were deterministic
* Which evidence was missing
* Why the system abstained

---

# 13. Confidence-Aware Automation

Initial policy:

```text
Confidence > 95%
        │
        ▼
AUTO-RECONCILE

75% – 95%
        │
        ▼
HUMAN REVIEW

< 75%
        │
        ▼
ESCALATE
```

Confidence is not treated as a guarantee.

The system evaluates confidence calibration against held-out ground truth.

A confidence score of `0.90` should ideally correspond to approximately 90% correctness over comparable predictions.

---

# 14. Human-in-the-Loop Learning

Human review is an active learning signal rather than a dead-end workflow.

When an operator resolves an exception, RAZE records the action.

Examples:

```text
ACCEPTED_AGENT_MATCH
OVERRIDDEN_AGENT_MATCH
MANUALLY_LINKED_RECORDS
CONFIRMED_EXCEPTION
REJECTED_CANDIDATE
```

The feedback is stored as structured training data.

```text
Agent Recommendation
        │
        ▼
Human Decision
        │
        ▼
Feedback Event
        │
        ├── Confidence Calibration
        │
        └── Policy Training Data
```

---

# 15. Experience Replay

Human decisions can be represented as experiences:

```text
(state, action, reward, outcome)
```

Example:

```text
State:
confidence = 0.91
amount_difference = 0
reference_similarity = 0.98

Agent Action:
AUTO_RECONCILE

Human:
ACCEPT

Outcome:
Correct Match

Reward:
+10
```

An incorrect autonomous recommendation can produce:

```text
Agent Action:
AUTO_RECONCILE

Human:
OVERRIDE

Outcome:
False Match

Reward:
-50
```

These experiences are stored in a prioritized replay dataset.

The system can prioritize:

* Human overrides
* High-value transactions
* Low-confidence decisions
* Rare exception types
* Incorrect autonomous matches

---

# 16. Offline Policy Improvement

Human feedback is **not immediately used to mutate the production policy**.

Instead:

```text
Human Feedback
      │
      ▼
Experience Store
      │
      ▼
Data Validation
      │
      ▼
Offline Training
      │
      ▼
Validation
      │
      ▼
Held-Out Evaluation
      │
      ▼
Policy Version
      │
      ▼
Controlled Deployment
```

This prevents a single incorrect human action or noisy feedback event from immediately changing autonomous financial behavior.

---

# 17. Confidence Calibration

Human feedback can also recalibrate the confidence estimator.

Example:

```text
Predicted Confidence: 92%
Observed Accuracy:    78%
```

The calibration layer identifies overconfidence.

The system can then retrain or recalibrate the confidence model using accumulated human-reviewed cases.

This addresses **confidence drift** as transaction patterns change.

---

# 18. Risk-Sensitive Reinforcement Learning

RL controls the intervention policy rather than financial truth.

### State

```text
amount_difference
amount_difference_ratio
date_difference
reference_similarity
merchant_similarity
fee_match
tax_match
historical_match_rate
candidate_count
confidence
transaction_value
exception_type
currency
settlement_type
refund_status
chargeback_status
```

### Actions

```text
AUTO_RECONCILE
REQUEST_REVIEW
INVESTIGATE
ESCALATE
```

### Reward

Initial experimental reward:

```text
Correct auto-reconciliation      +10
Correct review                    +3
Correct investigation             +3
Correct escalation                +5

Incorrect auto-reconciliation    -50

Unnecessary review                -2
Unnecessary investigation         -1
Unnecessary escalation            -3
```

The reward function is configurable and evaluated under different financial risk assumptions.

---

# 19. RL Environment

The policy is trained entirely against synthetic environments with known ground truth.

```text
                  Environment
                       │
                       ▼
                     State
                       │
                       ▼
                   RL Policy
                       │
                       ▼
                     Action
                       │
                       ▼
                  Ground Truth
                       │
                       ▼
                    Reward
                       │
                       ▼
                  Next State
```

No RL training occurs against live financial transactions.

---

# 20. RL Baselines

RAZE evaluates whether RL is actually necessary.

```text
Static Threshold
       │
       ▼
Supervised Policy
       │
       ▼
Contextual Bandit
       │
       ▼
PPO
```

The goal is to determine whether adaptive policies provide a measurable improvement over simpler approaches.

---

# 21. Benchmark Design

The benchmark contains clean and corrupted financial records.

Example distribution:

```text
10,000 Records

8,000 Normal
2,000 Corrupted
```

Corruption classes include:

```text
Amount mismatch
Missing settlement
Duplicate transaction
Date shift
Reference corruption
Partial settlement
Batched settlement
Refund
Chargeback
FX discrepancy
Fee discrepancy
Tax discrepancy
1-to-N mapping
N-to-1 mapping
```

The benchmark maintains ground-truth relationships for objective evaluation.

---

# 22. Evaluation Metrics

### Reconciliation Quality

* Precision
* Recall
* F1
* Match accuracy

### Financial Safety

* False Auto-Match Rate
* Incorrect Autonomous Action Rate
* High-value error rate

### Operational Efficiency

* Automation Rate
* Human Review Rate
* Exception Resolution Rate
* Throughput
* Latency
* Cost per record

### AI Reliability

* Confidence Calibration
* Abstention Quality
* Evidence Coverage
* Agent Failure Rate

---

# 23. Stress Testing

The Go control plane is benchmarked independently from the AI pipeline.

### Deterministic Engine

Measure:

```text
Records / second
Requests / second
p50 latency
p95 latency
p99 latency
CPU utilization
Memory utilization
```

### AI Pipeline

Measure:

```text
Agent investigations / second
Average investigation latency
p95 investigation latency
LLM calls / investigation
Inference cost
Failure rate
```

The goal is not to claim arbitrary throughput figures but to publish measurements from the deployed implementation.

---

# 24. Circuit Breaker

RAZZOR treats the AI service as a dependency that can fail or become slow.

If the Python service exceeds configured latency or error thresholds:

```text
Python AI Service
       │
       │ Timeout / Failure
       ▼
Go Circuit Breaker
       │
       ▼
Fallback Policy
       │
       ▼
HUMAN_REVIEW
```

The financial workflow remains available even if the AI service becomes unavailable.

The system fails **closed**, not open.

An unavailable AI model must not cause uncontrolled financial automation.

---

# 25. Asynchronous Agent Execution

Long-running investigations are processed asynchronously.

```text
Transaction
     │
     ▼
Go Control Plane
     │
     ▼
Redis / Asynq
     │
     ▼
Python Worker
     │
     ├── Retrieve Evidence
     ├── Match Candidates
     ├── Verify
     ├── Investigate
     └── Generate Decision
     │
     ▼
Go Control Plane
     │
     ▼
PostgreSQL
     │
     ▼
WebSocket
     │
     ▼
Next.js
```

Workflow states:

```text
PENDING
   ↓
MATCHING
   ↓
VERIFYING
   ↓
INVESTIGATING
   ↓
RESOLVED

or

INVESTIGATING
   ↓
ESCALATED
```

---

# 26. Concurrency Control

Financial state is managed transactionally in PostgreSQL.

RAZZOR uses:

* PostgreSQL transactions
* Optimistic locking
* Row-level locking where required
* Idempotency keys
* Versioned state transitions

Example:

```text
Transaction Version = 7

Worker A → reads 7
Worker B → reads 7

Worker A → commits 8

Worker B → detects stale version
         → rejects / retries
```

This prevents concurrent agents or human operators from applying conflicting financial state transitions.

---

# 27. Multi-Currency Reconciliation

Currency-aware reconciliation tracks:

```text
source_currency
settlement_currency
source_amount
settlement_amount
exchange_rate
conversion_timestamp
fx_difference
```

Example:

```text
Authorization
USD 100

FX Rate
₹83.20

Expected INR
₹8,320

Settlement
₹8,295

FX / Settlement Difference
₹25
```

The system distinguishes between:

* Genuine reconciliation errors
* Expected currency conversion differences
* Unexplained FX discrepancies

---

# 28. Refunds & Chargebacks

Financial state is not assumed to be immutable after initial reconciliation.

Example:

```text
Day 1

Payment
₹10,000

Settlement
₹9,823

→ RECONCILED
```

Later:

```text
Day 3

Partial Refund
₹3,000
```

RAZZOR creates a new adjustment event rather than silently rewriting the historical reconciliation.

```text
Original Reconciliation
        │
        ▼
Adjustment Event
        │
        ▼
Updated Financial Position
```

This maintains historical auditability.

---

# 29. Audit Trail

Every financial decision records:

```text
timestamp
reconciliation_id
transaction_id
source_records
agent
action
confidence
evidence
decision
previous_state
new_state
actor
policy_version
model_version
data_version
```

Example:

```text
REC_9021

Decision:
RECOMMEND_MATCH

Confidence:
0.92

Policy:
risk-policy-v3

Model:
investigator-v2

Evidence:
5 records

State:
INVESTIGATING → REVIEW
```

This enables reproducibility and post-decision analysis.

---

# 30. Technology Stack

| Layer               | Technology               |
| ------------------- | ------------------------ |
| Frontend            | Next.js                  |
| Language            | TypeScript               |
| UI                  | Tailwind CSS + shadcn/ui |
| Charts              | Recharts                 |
| Backend             | Go                       |
| Go HTTP             | Chi / Gin                |
| Internal RPC        | gRPC + Protocol Buffers  |
| Async Processing    | Redis + Asynq            |
| Database            | PostgreSQL               |
| Vector Search       | pgvector                 |
| AI Service          | Python + FastAPI         |
| Agent Orchestration | LangGraph                |
| ML                  | scikit-learn             |
| Deep Learning       | PyTorch                  |
| RL                  | Stable-Baselines3        |
| Matching            | RapidFuzz                |
| Embeddings          | Sentence Transformers    |
| Containerization    | Docker                   |

---

# 31. Repository Structure

```text
raze/
│
├── frontend/
│   ├── app/
│   ├── components/
│   ├── lib/
│   └── public/
│
├── services/
│   │
│   ├── api/
│   │   ├── cmd/
│   │   ├── internal/
│   │   │   ├── handlers/
│   │   │   ├── services/
│   │   │   ├── repositories/
│   │   │   ├── workflows/
│   │   │   ├── models/
│   │   │   └── middleware/
│   │   └── proto/
│   │
│   └── worker/
│
├── ai/
│   │
│   ├── agents/
│   │   ├── controller/
│   │   ├── matcher/
│   │   ├── verifier/
│   │   └── investigator/
│   │
│   ├── reconciliation/
│   │   ├── rules.py
│   │   ├── fuzzy.py
│   │   └── scoring.py
│   │
│   ├── evidence/
│   │   ├── schema.py
│   │   ├── graph.py
│   │   └── validator.py
│   │
│   ├── feedback/
│   │   ├── events.py
│   │   ├── replay.py
│   │   └── calibration.py
│   │
│   ├── rl/
│   │   ├── environment.py
│   │   ├── policy.py
│   │   ├── train.py
│   │   └── evaluate.py
│   │
│   └── evaluation/
│       ├── metrics.py
│       ├── benchmark.py
│       ├── stress.py
│       └── ablation.py
│
├── data/
│   ├── generator/
│   ├── corruption/
│   ├── raw/
│   ├── processed/
│   └── benchmark/
│
├── experiments/
│   ├── baselines/
│   ├── rl/
│   ├── calibration/
│   └── results/
│
├── database/
│   ├── migrations/
│   └── seeds/
│
├── proto/
│   └── finance.proto
│
├── docs/
│   ├── architecture/
│   ├── research/
│   └── evaluation/
│
├── docker-compose.yml
└── README.md
```

---

# 32. Security & Financial Controls

RAZE follows strict autonomy boundaries.

### Deterministic Financial Logic

Financial arithmetic and critical validation are deterministic.

### Evidence Requirement

Autonomous recommendations require supporting evidence.

### Bounded Agent Actions

Agents cannot arbitrarily mutate authoritative financial records.

### Confidence Gating

Low-confidence cases are routed to review or escalation.

### Human Control

Operators can approve, reject, or override recommendations.

### Idempotency

Repeated requests cannot silently create duplicate financial operations.

### Circuit Breaking

AI failures fall back to human review.

### Auditability

All state-changing operations are recorded.

### Test-Mode Operation

The buildathon implementation uses Razorpay Test Mode and synthetic data rather than production financial transactions.

---

# 33. Product Interface

RAZE provides a financial operations console rather than a chatbot.

### Control Center

Displays:

* Reconciliation volume
* Amount reconciled
* Exceptions
* Automation rate
* Precision
* False-match rate
* Human review load

### Reconciliation

Provides:

* Transaction matching
* Candidate records
* Financial calculations
* Confidence
* Evidence

### Investigation

Provides:

* Agent status
* Evidence graph
* Discrepancy analysis
* Recommendation
* Human controls

### Exceptions

Provides:

* Escalation queue
* Severity
* Exception category
* Evidence
* Resolution status

### Evaluation Lab

Provides:

* Benchmark results
* Baseline comparisons
* RL results
* Calibration
* Ablation studies
* Stress-test results

### Audit Trail

Provides:

* Decision history
* Evidence
* Agent/model versions
* Policy versions
* State transitions

---

# 34. End-to-End Decision Flow

```text
                 Transaction
                      │
                      ▼
              Data Normalization
                      │
                      ▼
             Deterministic Checks
                      │
                      ▼
              Candidate Retrieval
                      │
                      ▼
              Evidence Construction
                      │
                      ▼
             Agentic Investigation
                      │
                      ▼
             Confidence Estimation
                      │
                      ▼
               Risk Policy
                      │
          ┌───────────┼───────────┐
          ▼           ▼           ▼
       MATCH        REVIEW     ESCALATE
          │           │           │
          └───────────┼───────────┘
                      ▼
                 Human Feedback
                      │
                      ▼
            Experience / Calibration
                      │
                      ▼
                  Audit Trail
```

---

# 35. Design Philosophy

RAZE treats financial autonomy as a controlled systems problem.

**Deterministic systems establish truth.**

**Machine learning ranks possibilities.**

**Agents investigate ambiguity.**

**Evidence graphs make decisions auditable.**

**Confidence controls autonomy.**

**Human decisions provide learning signals.**

**Risk-sensitive policies optimize intervention.**

**Go protects the financial control plane.**

**Python provides the intelligence layer.**

**The system fails closed when AI becomes unavailable.**

> **Raze the backlog. Resolve the discrepancy. Prove the decision.**
