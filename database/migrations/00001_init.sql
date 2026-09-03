-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Normalized canonical financial records. All money in integer minor units (paise/cents).
CREATE TABLE records (
    id              BIGSERIAL PRIMARY KEY,
    source          TEXT        NOT NULL,          -- origin namespace, e.g. 'razorpay_payments'
    is_synthetic    BOOLEAN     NOT NULL DEFAULT FALSE, -- TRUE => generated demo data, never claims to be a real provider record
    external_id     TEXT        NOT NULL,          -- provider/generator identifier, e.g. 'PAY_001'
    kind            TEXT        NOT NULL,          -- payment | settlement | refund | fee | tax | chargeback
    status          TEXT        NOT NULL DEFAULT 'active',
    amount_minor    BIGINT      NOT NULL,          -- gross amount in minor units
    fee_minor       BIGINT      NOT NULL DEFAULT 0,
    tax_minor       BIGINT      NOT NULL DEFAULT 0,
    net_minor       BIGINT      NOT NULL DEFAULT 0, -- amount - fee - tax
    currency        TEXT        NOT NULL DEFAULT 'INR',
    occurred_at     TIMESTAMPTZ NOT NULL,
    ref_external_id TEXT,                          -- optional cross-reference, e.g. settlement -> payment
    truth_group     TEXT,                          -- synthetic ground-truth key for benchmark evaluation only
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_records_source_external UNIQUE (source, external_id)
);
CREATE INDEX idx_records_kind      ON records (kind);
CREATE INDEX idx_records_occurred  ON records (occurred_at);
CREATE INDEX idx_records_ref       ON records (ref_external_id) WHERE ref_external_id IS NOT NULL;

-- Reconciliation run / job.
CREATE TABLE reconciliation_jobs (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'PENDING', -- PENDING|RUNNING|COMPLETED|FAILED
    config        JSONB       NOT NULL DEFAULT '{}',
    total_records BIGINT      NOT NULL DEFAULT 0,
    matched       BIGINT      NOT NULL DEFAULT 0,
    review        BIGINT      NOT NULL DEFAULT 0,
    escalated     BIGINT      NOT NULL DEFAULT 0,
    version       BIGINT      NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ
);

-- Per-record workflow unit. Each item reconciles one record against a matched record (if any).
CREATE TABLE reconciliation_items (
    id              BIGSERIAL PRIMARY KEY,
    job_id          BIGINT      NOT NULL REFERENCES reconciliation_jobs(id) ON DELETE CASCADE,
    record_id       BIGINT      NOT NULL REFERENCES records(id),
    match_record_id BIGINT      REFERENCES records(id),
    status          TEXT        NOT NULL DEFAULT 'PENDING', -- PENDING|MATCHING|VERIFYING|INVESTIGATING|REVIEW|RESOLVED|ESCALATED|FAILED
    decision        TEXT,                                  -- RECONCILED|REVIEW|ESCALATE
    confidence      NUMERIC(5,4),
    version         BIGINT      NOT NULL DEFAULT 1,         -- optimistic lock
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ
);
CREATE INDEX idx_items_job    ON reconciliation_items (job_id);
CREATE INDEX idx_items_status ON reconciliation_items (status);
CREATE INDEX idx_items_record ON reconciliation_items (record_id);

-- Candidate relationships proposed during matching.
CREATE TABLE candidates (
    id               BIGSERIAL PRIMARY KEY,
    item_id          BIGINT       NOT NULL REFERENCES reconciliation_items(id) ON DELETE CASCADE,
    target_record_id BIGINT       NOT NULL REFERENCES records(id),
    strategy         TEXT         NOT NULL,  -- exact_reference | amount_similarity | timestamp_proximity | semantic
    similarity       NUMERIC(6,4) NOT NULL DEFAULT 0,
    score            NUMERIC(6,4) NOT NULL DEFAULT 0,
    status           TEXT         NOT NULL DEFAULT 'proposed', -- proposed|ranked|accepted|rejected
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_candidates_item   ON candidates (item_id);
CREATE INDEX idx_candidates_target ON candidates (target_record_id);

-- Structured, machine-verifiable evidence per item/candidate.
CREATE TABLE evidence (
    id           BIGSERIAL PRIMARY KEY,
    item_id      BIGINT       NOT NULL REFERENCES reconciliation_items(id) ON DELETE CASCADE,
    candidate_id BIGINT       REFERENCES candidates(id) ON DELETE SET NULL,
    type         TEXT         NOT NULL,   -- EXACT_REFERENCE_MATCH|AMOUNT_EXPLAINED|FEE_BREAKDOWN_EXPLAINED|TAX_EXPLAINED|DATE_PROXIMITY|SEMANTIC_SIMILARITY
    weight       NUMERIC(4,3) NOT NULL DEFAULT 0,
    details      JSONB        NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_evidence_item ON evidence (item_id);

-- Immutable audit trail. Append-only by application convention (no UPDATE/DELETE grants).
CREATE TABLE audit_events (
    id         BIGSERIAL PRIMARY KEY,
    item_id    BIGINT      REFERENCES reconciliation_items(id) ON DELETE SET NULL,
    actor      TEXT        NOT NULL,       -- 'system' | 'worker' | 'human:<email>'
    action     TEXT        NOT NULL,
    entity     TEXT        NOT NULL,
    before     JSONB       NOT NULL DEFAULT '{}',
    after      JSONB       NOT NULL DEFAULT '{}',
    metadata   JSONB       NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_item ON audit_events (item_id);

-- Operator review decisions (structured feedback for learning).
CREATE TABLE human_review_actions (
    id         BIGSERIAL PRIMARY KEY,
    item_id    BIGINT      NOT NULL REFERENCES reconciliation_items(id) ON DELETE CASCADE,
    action     TEXT        NOT NULL,   -- ACCEPTED_AGENT_MATCH|REJECTED_CANDIDATE|MANUALLY_LINKED_RECORDS|CONFIRMED_EXCEPTION|ESCALATED
    actor      TEXT        NOT NULL,
    note       TEXT        NOT NULL DEFAULT '',
    evidence   JSONB       NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_review_item ON human_review_actions (item_id);

-- Recommendations from the Python AI service. Advisory only: the Go control plane
-- decides whether to apply anything. Stored for auditability and reproducibility.
CREATE TABLE ai_decisions (
    id                BIGSERIAL PRIMARY KEY,
    item_id           BIGINT      NOT NULL REFERENCES reconciliation_items(id) ON DELETE CASCADE,
    recommendation    TEXT        NOT NULL,  -- RECOMMEND_MATCH|REQUEST_REVIEW|ESCALATE
    confidence        NUMERIC(5,4) NOT NULL,
    candidate_rankings JSONB      NOT NULL DEFAULT '[]',
    investigation     JSONB       NOT NULL DEFAULT '{}',
    model_version     TEXT        NOT NULL DEFAULT 'unknown',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_item ON ai_decisions (item_id);

-- Idempotency for safe re-submission of state-changing requests.
CREATE TABLE idempotency_keys (
    key          TEXT PRIMARY KEY,
    request_hash TEXT        NOT NULL,
    response     JSONB       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL DEFAULT now() + interval '24 hours'
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS ai_decisions;
DROP TABLE IF EXISTS human_review_actions;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS evidence;
DROP TABLE IF EXISTS candidates;
DROP TABLE IF EXISTS reconciliation_items;
DROP TABLE IF EXISTS reconciliation_jobs;
DROP TABLE IF EXISTS records;
-- +goose StatementEnd
