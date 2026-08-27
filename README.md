# RAZE

### Autonomous Financial Reconciliation & Risk-Aware Control

> **Raze reconciliation backlogs. Resolve financial discrepancies. Prove every decision.**

RAZE is an agentic financial reconciliation and control system designed to reconcile heterogeneous payment and settlement records, investigate discrepancies, quantify uncertainty, and determine when a financial operation can be safely automated or requires human intervention.

Built for the **Razorpay AI Finance Controller** track using **Razorpay Test Mode and controlled synthetic financial data**.

---

## Table of Contents

* [Why RAZE](#why-raze)
* [Core Principle](#core-principle)
* [The Problem](#the-problem)
* [What RAZE Does](#what-raze-does)
* [Architecture](#architecture)
* [Reconciliation Pipeline](#reconciliation-pipeline)
* [Deterministic Financial Verification](#deterministic-financial-verification)
* [Multi-Record Reconciliation](#multi-record-reconciliation)
* [Agentic Investigation](#agentic-investigation)
* [Evidence & Explainability](#evidence--explainability)
* [Confidence-Aware Automation](#confidence-aware-automation)
* [Human-in-the-Loop Learning](#human-in-the-loop-learning)
* [Risk-Sensitive Policy Learning](#risk-sensitive-policy-learning)
* [Product Experience](#product-experience)
* [Benchmark & Evaluation](#benchmark--evaluation)
* [System Reliability](#system-reliability)
* [Technology Stack](#technology-stack)
* [Repository Structure](#repository-structure)
* [Security & Financial Controls](#security--financial-controls)
* [Deployment](#deployment)
* [Roadmap](#roadmap)

---

# Why RAZE?

Financial operations rarely exist in one clean database.

A real reconciliation workflow may involve:

* Payments
* Orders
* Settlements
* Processing fees
* Taxes
* Refunds
* Chargebacks
* Merchant ledgers
* Batched settlements
* Partial settlements
* Multiple currencies
* Missing or duplicated records

Traditional rule-based reconciliation works well when records are clean and relationships are obvious.

The problem begins when reality is ambiguous.

```text
Payment A ──────┐
Payment B ──────┼────── Settlement Batch
Payment C ──────┘

        ↓

Is this relationship correct?

Are the fees explained?

Are taxes correct?

Is there a missing record?

Should this be automatically reconciled?

Or should a human intervene?
```

RAZE treats this as a **controlled decision system**, not simply a matching problem.

---

# Core Principle

> **Trustworthy autonomy over maximum automation.**

RAZE separates authoritative financial logic from probabilistic intelligence.

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
                   Risk Policy
                         │
            ┌────────────┼────────────┐
            ▼            ▼            ▼
         MATCH         REVIEW      ESCALATE
            │            │            │
            └────────────┼────────────┘
                         │
                         ▼
                  Human Feedback
                         │
                         ▼
              Learning & Calibration
                         │
                         ▼
                    Audit Trail
```

The fundamental rule is:

> **Deterministic systems establish financial correctness. AI investigates ambiguity. Humans retain control over uncertainty.**

---

# The Problem

Financial reconciliation is fundamentally a verification problem.

RAZE handles situations including:

| Problem              | Example                                     |
| -------------------- | ------------------------------------------- |
| Missing records      | Payment exists but settlement is missing    |
| Duplicate records    | Same transaction appears multiple times     |
| Amount mismatch      | ₹10,000 payment → ₹9,800 settlement         |
| Partial settlement   | One payment settles across multiple records |
| Batched settlement   | Multiple payments map to one settlement     |
| Reference corruption | IDs differ slightly across systems          |
| Refund               | Reconciled payment is partially refunded    |
| Chargeback           | Transaction is adjusted after settlement    |
| FX differences       | Authorization in USD, settlement in INR     |
| Fee mismatch         | Unexpected processing fee                   |
| Tax mismatch         | Incorrect tax calculation                   |

The challenge is not simply:

```text
"Find the closest transaction."
```

It is:

```text
"Can we prove that these financial records represent
the same underlying financial event?"
```

---

# What RAZE Does

RAZE combines multiple layers of intelligence.

```text
                     RAW RECORDS
                          │
                          ▼
                  Normalization
                          │
                          ▼
              Deterministic Validation
                          │
                          ▼
               Candidate Generation
                 │       │       │
                 ▼       ▼       ▼
               Exact    Fuzzy  Semantic
                 │       │       │
                 └───────┼───────┘
                         ▼
                  Candidate Ranking
                         │
                         ▼
                Evidence Construction
                         │
                         ▼
                Agent Investigation
                         │
                         ▼
               Confidence Estimation
                         │
                         ▼
                   Risk Policy
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
       MATCH          REVIEW         ESCALATE
```

---

# Architecture

RAZE uses a deliberately separated architecture.

```text
┌──────────────────────────────────────────────┐
│                                              │
│            RAZE Operations Interface         │
│                                              │
│   Import → Reconcile → Investigate → Review  │
│                                              │
└──────────────────────┬───────────────────────┘
                       │
                 REST / WebSocket
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                                              │
│               Go Control Plane               │
│                                              │
│  • API                                       │
│  • Workflow State                            │
│  • Financial Mutations                       │
│  • Authorization                             │
│  • Audit                                     │
│  • Idempotency                               │
│  • Concurrency Control                       │
│  • Circuit Breaker                           │
│                                              │
└───────────────┬──────────────────┬───────────┘
                │                  │
               gRPC           PostgreSQL
                │                  │
                ▼                  ▼
┌──────────────────────────┐   ┌───────────────┐
│     Python AI/ML Layer    │   │   pgvector    │
│                          │   └───────────────┘
│ • Matching               │
│ • Retrieval              │
│ • Agents                 │
│ • Evidence               │
│ • Confidence             │
│ • Policy Learning        │
│ • Evaluation             │
│                          │
└──────────────┬───────────┘
               │
               ▼
┌──────────────────────────────────────────────┐
│              Financial Data Sources          │
│                                              │
│  Razorpay Test Mode + Synthetic Benchmark    │
│             + Human Feedback                 │
└──────────────────────────────────────────────┘
```

---

# Architectural Boundaries

## Go Control Plane

Go owns the **authoritative financial state**.

Responsibilities:

* HTTP API
* Request validation
* Workflow orchestration
* Database transactions
* State transitions
* Authorization
* Financial mutations
* Audit logging
* Idempotency
* Concurrency control
* Async job submission
* Circuit breaking
* WebSocket events

The control plane decides whether an action is legally allowed within the system.

---

## Python AI/ML Layer

Python owns probabilistic and research components.

Responsibilities:

* Candidate generation
* Fuzzy matching
* Semantic retrieval
* Agentic investigation
* Evidence construction
* Confidence estimation
* Policy inference
* Offline policy training
* Benchmark generation
* Evaluation
* Calibration

### Important Boundary

```text
Python can recommend.

Python cannot directly mutate
authoritative financial state.
```

All state-changing operations must pass through the Go control plane.

---

# Reconciliation Pipeline

## 1. Schema Normalization

Different sources use different schemas.

```text
Payment Provider

{
  payment_id,
  amount,
  currency,
  created_at
}

Settlement System

{
  settlement_id,
  net_amount,
  settlement_date
}
```

RAZE normalizes these records into a canonical financial representation.

---

## 2. Deterministic Checks

Financial calculations are performed deterministically.

Example:

```text
Payment                  ₹10,000
Processing Fee              ₹150
Tax                          ₹27
────────────────────────────────
Expected Settlement       ₹9,823

Actual Settlement         ₹9,823
────────────────────────────────
Difference                    ₹0
```

Result:

```text
RECONCILED
```

An LLM is never allowed to override arithmetic inconsistencies.

---

## 3. Candidate Generation

RAZE generates possible relationships using multiple strategies.

```text
                    Financial Record
                          │
            ┌─────────────┼─────────────┐
            ▼             ▼             ▼
          Exact         Fuzzy        Semantic
            │             │             │
            └─────────────┼─────────────┘
                          ▼
                    Candidates
```

Signals may include:

* Exact transaction references
* Amount similarity
* Timestamp proximity
* Merchant similarity
* Currency compatibility
* Settlement structure
* Historical patterns

---

## 4. Evidence Verification

Candidates are not trusted merely because they look similar.

RAZE verifies:

```text
Reference Match?
        │
Amount Explained?
        │
Fees Explained?
        │
Taxes Explained?
        │
Settlement Structure Valid?
        │
Currency Conversion Valid?
        │
Missing Evidence?
```

The result becomes structured evidence.

---

# Multi-Record Reconciliation

RAZE does not assume financial records have a simple 1:1 relationship.

Supported relationships:

```text
1 Payment  → 1 Settlement

N Payments → 1 Settlement

1 Payment  → N Settlement Components

N Payments → N Settlement Components
```

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
       Fee      ₹400
       Tax       ₹72
             │
             ▼
        Net ₹19,528
```

The engine determines whether the settlement can be fully explained by the underlying financial records.

---

# Edge Cases

RAZE explicitly models difficult financial scenarios.

## Amount Errors

```text
Expected: ₹10,000

Actual:    ₹9,800
```

## Missing Records

```text
Payment exists

Settlement missing
```

## Duplicate Records

```text
PAY_1001
PAY_1001
```

## Partial Settlement

```text
Payment      ₹20,000

Settlement A ₹12,000
Settlement B  ₹8,000
```

## Refunds

```text
Original Payment
       ↓
Reconciled
       ↓
Partial Refund
       ↓
Adjustment Event
```

## Multi-Currency

```text
Authorization

USD 100

FX Rate

₹83.20

Expected

₹8,320

Actual Settlement

₹8,295

Difference

₹25
```

RAZE distinguishes between:

* Expected FX variation
* Settlement timing differences
* Genuine reconciliation errors

---

# Agentic Investigation

RAZE uses specialized bounded agents.

It does **not** use an unrestricted autonomous agent.

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
             ┌──────────┴──────────┐
             ▼                     ▼
          Resolve               Escalate
```

## Matcher

Finds plausible candidate records.

## Verifier

Checks:

* Numerical consistency
* Structural consistency
* Financial relationships
* Deterministic rules

## Investigator

Explores unresolved ambiguity using available evidence.

## Controller

Coordinates the investigation and enforces bounded execution.

---

# Evidence & Explainability

RAZE does not rely on free-form explanations as the source of truth.

Every investigation produces a structured decision object.

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
        "tax": 36
      },
      "unexplained_delta": 0,
      "weight": 0.5
    }
  ],
  "abstention_reasons": []
}
```

This allows the frontend and audit system to render evidence directly.

---

# Evidence Graph

Evidence relationships are represented as a graph.

```text
PAY_10023
    │
    │ EXACT_REFERENCE_MATCH
    ▼
SETTLE_8801
    │
    ├──── FEE_RECORD
    │         │
    │         ▼
    │       ₹200
    │
    └──── TAX_RECORD
              │
              ▼
             ₹36

              ↓

       FINANCIAL MATCH
```

Operators can inspect:

* Which records contributed
* Which relationships were exact
* Which relationships were inferred
* Which calculations were deterministic
* Which evidence was missing
* Why automation was rejected

---

# Confidence-Aware Automation

RAZE treats confidence as a decision signal, not a guarantee.

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

Confidence must be calibrated.

For example:

```text
Predicted Confidence: 92%

Observed Accuracy:    78%
```

This indicates overconfidence.

RAZE continuously evaluates calibration against held-out ground truth.

---

# Human-in-the-Loop Learning

Human review is not a dead end.

Every operator action becomes structured feedback.

Examples:

```text
ACCEPTED_AGENT_MATCH

OVERRIDDEN_AGENT_MATCH

MANUALLY_LINKED_RECORDS

CONFIRMED_EXCEPTION

REJECTED_CANDIDATE
```

Feedback pipeline:

```text
Agent Recommendation
        │
        ▼
Human Decision
        │
        ▼
Feedback Event
        │
        ├──────────────► Confidence Calibration
        │
        └──────────────► Policy Training Dataset
```

Human decisions are stored as learning signals.

They do **not** immediately modify production behavior.

---

# Offline Policy Improvement

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

This prevents noisy feedback or a single incorrect decision from immediately changing autonomous financial behavior.

---

# Risk-Sensitive Policy Learning

The policy controls **intervention decisions**, not financial truth.

## State

Example signals:

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

## Actions

```text
AUTO_RECONCILE

REQUEST_REVIEW

INVESTIGATE

ESCALATE
```

## Example Reward Design

```text
Correct Auto-Reconciliation      +10
Correct Review                    +3
Correct Investigation             +3
Correct Escalation                +5

Incorrect Auto-Reconciliation    -50

Unnecessary Review                -2
Unnecessary Investigation         -1
Unnecessary Escalation            -3
```

The heavy penalty for incorrect autonomous reconciliation reflects asymmetric financial risk.

---

# Policy Baselines

RAZE evaluates whether complex learning is actually necessary.

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
Adaptive RL Policy
```

The objective is not to claim that RL is automatically superior.

The objective is to measure whether adaptive policies improve the automation-risk trade-off.

---

# Product Experience

## Not Another Dashboard

RAZE is intentionally designed as a **workflow-first financial operations application**.

The product is not centered around:

* KPI cards
* Generic charts
* Analytics dashboards
* Decorative metrics

Instead, the interface follows the actual reconciliation workflow.

```text
Import Records
      │
      ▼
Run Reconciliation
      │
      ▼
Review Results
      │
      ▼
Investigate Exceptions
      │
      ▼
Inspect Evidence
      │
      ▼
Approve / Reject / Escalate
      │
      ▼
Audit Record
```

---

## Core Screens

### 1. Reconciliation Jobs

The entry point for reconciliation.

Operators can:

* Import financial records
* Select reconciliation sources
* Start a reconciliation run
* Track processing state
* View completed runs

Example:

```text
RECONCILIATION JOB #REC-1042

Status: INVESTIGATING

Records: 12,481

Matched: 11,902

Review Required: 421

Escalated: 158
```

---

### 2. Reconciliation Results

Displays the actual output of a reconciliation run.

```text
┌──────────────────────────────────────────────┐
│ PAYMENT          SETTLEMENT       STATUS     │
├──────────────────────────────────────────────┤
│ PAY_1023         SET_8821         MATCHED    │
│ PAY_1024         SET_8822         REVIEW     │
│ PAY_1025         —                ESCALATED  │
└──────────────────────────────────────────────┘
```

Operators can drill directly into individual cases.

---

### 3. Investigation Workspace

This is the core product experience.

```text
┌─────────────────────────────────────────────┐
│ REC_9021                                   │
│                                             │
│ Status: REVIEW REQUIRED                     │
│ Confidence: 92%                             │
│                                             │
├─────────────────────────────────────────────┤
│                                             │
│ Candidate Records                           │
│                                             │
│ PAY_10023     Similarity: 98%              │
│ PAY_10027     Similarity: 74%              │
│                                             │
├─────────────────────────────────────────────┤
│                                             │
│ Evidence                                   │
│                                             │
│ ✓ Exact reference match                    │
│ ✓ Amount explained                         │
│ ✓ Fee breakdown verified                   │
│ ✓ Tax calculation verified                 │
│                                             │
├─────────────────────────────────────────────┤
│                                             │
│ Recommendation                             │
│                                             │
│ REQUEST_REVIEW                              │
│                                             │
│ [Approve] [Reject] [Investigate] [Escalate] │
└─────────────────────────────────────────────┘
```

The operator sees the actual evidence behind the recommendation.

---

### 4. Human Review

Operators can:

* Accept recommendation
* Reject recommendation
* Manually link records
* Request additional investigation
* Escalate exceptions

Every action generates a structured feedback event.

---

### 5. Transaction Audit

Each reconciliation has a complete history.

```text
REC_9021

10:02  CREATED

10:03  MATCHING

10:03  CANDIDATES GENERATED

10:04  EVIDENCE VERIFIED

10:05  AGENT INVESTIGATION

10:06  REQUEST_REVIEW

10:10  HUMAN ACCEPTED

10:10  RESOLVED
```

The audit record stores:

* Decision
* Evidence
* Agent version
* Model version
* Policy version
* Previous state
* New state
* Human actions
* Timestamp

---

# Benchmark & Evaluation

RAZE uses controlled synthetic financial data with known ground truth.

Example benchmark:

```text
10,000 Financial Records

8,000 Normal

2,000 Corrupted
```

Corruption classes:

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

---

# Evaluation Metrics

## Reconciliation Quality

* Precision
* Recall
* F1
* Match accuracy

## Financial Safety

* False Auto-Match Rate
* Incorrect Autonomous Action Rate
* High-value error rate

## Operational Efficiency

* Automation Rate
* Human Review Rate
* Exception Resolution Rate
* Throughput
* Latency
* Cost per record

## AI Reliability

* Confidence calibration
* Abstention quality
* Evidence coverage
* Agent failure rate

---

# Stress Testing

The deterministic engine and AI pipeline are benchmarked independently.

## Deterministic Engine

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

## AI Investigation Pipeline

Measure:

```text
Investigations / second
Average latency
p95 latency
LLM calls / investigation
Inference cost
Failure rate
```

RAZE publishes measured results rather than arbitrary performance claims.

---

# Asynchronous Execution

Agent investigations may take longer than deterministic checks.

Therefore, long-running workflows are asynchronous.

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
WebSocket Event
     │
     ▼
Operations Interface
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
```

Or:

```text
INVESTIGATING
      ↓
ESCALATED
```

---

# Circuit Breaker

The AI system is treated as an external dependency that can fail.

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
HUMAN REVIEW
```

RAZE fails **closed**, not open.

An unavailable AI model must never result in uncontrolled financial automation.

---

# Concurrency Control

Financial state is managed transactionally.

RAZE uses:

* PostgreSQL transactions
* Optimistic locking
* Row-level locking where required
* Idempotency keys
* Versioned state transitions

Example:

```text
Transaction Version = 7

Worker A → reads version 7
Worker B → reads version 7

Worker A → commits version 8

Worker B → stale version detected
         → retry / reject
```

This prevents conflicting state changes from agents or human operators.

---

# Audit Trail

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

State:
INVESTIGATING → REVIEW
```

---

# Technology Stack

| Layer               | Technology               |
| ------------------- | ------------------------ |
| Frontend            | Next.js                  |
| Language            | TypeScript               |
| UI                  | Tailwind CSS + shadcn/ui |
| Backend             | Go                       |
| HTTP                | Chi / Gin                |
| Internal RPC        | gRPC + Protocol Buffers  |
| Async Processing    | Redis + Asynq            |
| Database            | PostgreSQL               |
| Vector Search       | pgvector                 |
| AI Service          | Python + FastAPI         |
| Agent Orchestration | LangGraph                |
| Machine Learning    | scikit-learn             |
| Deep Learning       | PyTorch                  |
| Policy Learning     | Stable-Baselines3        |
| Fuzzy Matching      | RapidFuzz                |
| Embeddings          | Sentence Transformers    |
| Containerization    | Docker                   |

---

# Repository Structure

```text
raze/
│
├── frontend/
│   ├── app/
│   │   ├── jobs/
│   │   ├── reconciliation/
│   │   ├── investigation/
│   │   └── review/
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
│   ├── policy/
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
│   ├── policy/
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
├── .env.example
└── README.md
```

---

# Security & Financial Controls

RAZE enforces strict autonomy boundaries.

## Deterministic Financial Logic

Financial arithmetic and critical validation remain deterministic.

## Evidence Requirement

Autonomous recommendations require supporting evidence.

## Bounded Agent Actions

Agents cannot directly mutate financial records.

## Confidence Gating

Low-confidence decisions are routed to review or escalation.

## Human Control

Operators can approve, reject, override, or manually resolve recommendations.

## Idempotency

Repeated requests cannot silently create duplicate financial operations.

## Circuit Breaking

AI failures fall back to safe human review.

## Auditability

Every state-changing operation is recorded.

## Test-Mode Operation

The buildathon implementation uses:

* Razorpay Test Mode
* Controlled synthetic financial data

No production financial transactions are required.

---

# End-to-End Decision Flow

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
       MATCH       REVIEW     ESCALATE
         │           │           │
         └───────────┼───────────┘
                     ▼
               Human Feedback
                     │
                     ▼
          Experience & Calibration
                     │
                     ▼
                Audit Trail
```

---

# Deployment

The system is designed to run as independent services.

```text
                    Internet
                       │
                       ▼
                Next.js Frontend
                       │
                       ▼
                  Go API Service
                  │           │
                  ▼           ▼
             PostgreSQL      Redis
                  │           │
                  │           ▼
                  │       Asynq Worker
                  │           │
                  ▼           ▼
               pgvector   Python AI Service
```

Services are containerized using Docker.

Example:

```bash
git clone https://github.com/aksayush2005/raze.git

cd raze

cp .env.example .env

docker compose up --build
```

---


# Design Philosophy

RAZE treats financial autonomy as a **controlled systems problem**.

**Deterministic systems establish truth.**

**Machine learning ranks possibilities.**

**Agents investigate ambiguity.**

**Evidence makes decisions auditable.**

**Confidence controls autonomy.**

**Humans resolve uncertainty.**

**Feedback improves future decisions.**

**Adaptive policies optimize intervention.**

**Go protects the financial control plane.**

**Python provides probabilistic intelligence.**

**The system fails closed when AI becomes unavailable.**

---

# About

RAZE is an agentic financial reconciliation system combining:

* Deterministic financial controls
* Evidence-grounded AI investigation
* Confidence-aware automation
* Human-in-the-loop decision making
* Risk-sensitive policy learning
* Multi-record reconciliation
* Auditability
* Production-oriented distributed systems

> **Raze the backlog. Resolve the discrepancy. Prove the decision.**

---

## License

Apache-2.0
