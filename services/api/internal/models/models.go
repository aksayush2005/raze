package models

import "time"

// Record is a normalized canonical financial record.
// All money fields are integer minor units.
type Record struct {
	ID            int64     `json:"id"`
	Source        string    `json:"source"`
	IsSynthetic   bool      `json:"is_synthetic"`
	ExternalID    string    `json:"external_id"`
	Kind          string    `json:"kind"`
	Status        string    `json:"status"`
	AmountMinor   int64     `json:"amount_minor"`
	FeeMinor      int64     `json:"fee_minor"`
	TaxMinor      int64     `json:"tax_minor"`
	NetMinor      int64     `json:"net_minor"`
	Currency      string    `json:"currency"`
	OccurredAt    time.Time `json:"occurred_at"`
	RefExternalID *string   `json:"ref_external_id,omitempty"`
	TruthGroup    *string   `json:"truth_group,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Job is a reconciliation run.
type Job struct {
	ID           int64          `json:"id"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	Config       map[string]any `json:"config"`
	TotalRecords int64          `json:"total_records"`
	Matched      int64          `json:"matched"`
	Review       int64          `json:"review"`
	Escalated    int64          `json:"escalated"`
	Version      int64          `json:"version"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
}

// Item is a reconciliation workflow unit for one record.
type Item struct {
	ID            int64      `json:"id"`
	JobID         int64      `json:"job_id"`
	RecordID      int64      `json:"record_id"`
	MatchRecordID *int64     `json:"match_record_id,omitempty"`
	Status        string     `json:"status"`
	Decision      *string    `json:"decision,omitempty"`
	Confidence    *float64   `json:"confidence,omitempty"`
	Version       int64      `json:"version"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

// Candidate is a proposed relationship between the item's record and a target.
type Candidate struct {
	ID             int64     `json:"id"`
	ItemID         int64     `json:"item_id"`
	TargetRecordID int64     `json:"target_record_id"`
	Strategy       string    `json:"strategy"`
	Similarity     float64   `json:"similarity"`
	Score          float64   `json:"score"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// Evidence is structured, machine-verifiable support for a decision.
type Evidence struct {
	ID          int64          `json:"id"`
	ItemID      int64          `json:"item_id"`
	CandidateID *int64         `json:"candidate_id,omitempty"`
	Type        string         `json:"type"`
	Weight      float64        `json:"weight"`
	Details     map[string]any `json:"details"`
	CreatedAt   time.Time      `json:"created_at"`
}

// AuditEvent is an immutable record of a state-changing operation.
type AuditEvent struct {
	ID        int64          `json:"id"`
	ItemID    *int64         `json:"item_id,omitempty"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Entity    string         `json:"entity"`
	Before    map[string]any `json:"before"`
	After     map[string]any `json:"after"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

// HumanReviewAction is a structured operator decision.
type HumanReviewAction struct {
	ID        int64          `json:"id"`
	ItemID    int64          `json:"item_id"`
	Action    string         `json:"action"`
	Actor     string         `json:"actor"`
	Note      string         `json:"note"`
	Evidence  map[string]any `json:"evidence"`
	CreatedAt time.Time      `json:"created_at"`
}

// AIDecision is an advisory recommendation from the Python service.
// It is stored for auditability but never applied directly by itself.
type AIDecision struct {
	ID                int64            `json:"id"`
	ItemID            int64            `json:"item_id"`
	Recommendation    string           `json:"recommendation"`
	Confidence        float64          `json:"confidence"`
	CandidateRankings []map[string]any `json:"candidate_rankings"`
	Investigation     map[string]any   `json:"investigation"`
	ModelVersion      string           `json:"model_version"`
	CreatedAt         time.Time        `json:"created_at"`
}

// ItemView is an item joined with its record for list views.
type ItemView struct {
	Item
	RecordExternalID string `json:"record_external"`
	RecordKind       string `json:"kind"`
	AmountMinor      int64  `json:"amount_minor"`
}

// ItemStatus / decision constants.
const (
	StatusPending       = "PENDING"
	StatusMatching      = "MATCHING"
	StatusVerifying     = "VERIFYING"
	StatusInvestigating = "INVESTIGATING"
	StatusReview        = "REVIEW"
	StatusResolved      = "RESOLVED"
	StatusEscalated     = "ESCALATED"
	StatusFailed        = "FAILED"

	// JobStatusRunning marks a job whose items are being consumed by workers.
	// It is distinct from the per-item StatusMatching.
	StatusRunning = "RUNNING"

	DecisionReconciled = "RECONCILED"
	DecisionReview     = "REVIEW"
	DecisionEscalate   = "ESCALATE"
)
