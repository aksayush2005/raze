package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

// --- Candidates ------------------------------------------------------------

func (s *Store) CreateCandidates(ctx context.Context, cs []*models.Candidate) error {
	for _, c := range cs {
		if err := s.pool.QueryRow(ctx, `
			INSERT INTO candidates (item_id, target_record_id, strategy, similarity, score, status)
			VALUES ($1,$2,$3,$4,$5,'proposed') RETURNING id, created_at`,
			c.ItemID, c.TargetRecordID, c.Strategy, c.Similarity, c.Score).
			Scan(&c.ID, &c.CreatedAt); err != nil {
			return fmt.Errorf("create candidate: %w", err)
		}
	}
	return nil
}

func (s *Store) ListCandidatesByItem(ctx context.Context, itemID int64) ([]*models.Candidate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, item_id, target_record_id, strategy, similarity, score, status, created_at
		FROM candidates WHERE item_id = $1 ORDER BY score DESC`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Candidate
	for rows.Next() {
		var c models.Candidate
		if err := rows.Scan(&c.ID, &c.ItemID, &c.TargetRecordID, &c.Strategy,
			&c.Similarity, &c.Score, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// --- Evidence --------------------------------------------------------------

func (s *Store) CreateEvidence(ctx context.Context, evs []*models.Evidence) error {
	for _, e := range evs {
		if err := s.pool.QueryRow(ctx, `
			INSERT INTO evidence (item_id, candidate_id, type, weight, details)
			VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`,
			e.ItemID, e.CandidateID, e.Type, e.Weight, e.Details).
			Scan(&e.ID, &e.CreatedAt); err != nil {
			return fmt.Errorf("create evidence: %w", err)
		}
	}
	return nil
}

func (s *Store) ListEvidenceByItem(ctx context.Context, itemID int64) ([]*models.Evidence, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, item_id, candidate_id, type, weight, details, created_at
		FROM evidence WHERE item_id = $1 ORDER BY id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Evidence
	for rows.Next() {
		var e models.Evidence
		if err := rows.Scan(&e.ID, &e.ItemID, &e.CandidateID, &e.Type, &e.Weight, &e.Details, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// --- Audit -----------------------------------------------------------------

func (s *Store) CreateAuditEvent(ctx context.Context, e *models.AuditEvent) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO audit_events (item_id, actor, action, entity, before, after, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at`,
		e.ItemID, e.Actor, e.Action, e.Entity, e.Before, e.After, e.Metadata).
		Scan(&id, &e.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create audit: %w", err)
	}
	return id, nil
}

func (s *Store) ListAuditByItem(ctx context.Context, itemID int64) ([]*models.AuditEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, item_id, actor, action, entity, before, after, metadata, created_at
		FROM audit_events WHERE item_id = $1 ORDER BY id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.AuditEvent
	for rows.Next() {
		var e models.AuditEvent
		if err := rows.Scan(&e.ID, &e.ItemID, &e.Actor, &e.Action, &e.Entity,
			&e.Before, &e.After, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// AuditItem records a state-changing event scoped to a reconciliation item.
func (s *Store) AuditItem(ctx context.Context, itemID int64, actor, action string, metadata map[string]any) error {
	_, err := s.CreateAuditEvent(ctx, &models.AuditEvent{
		ItemID:   &itemID,
		Actor:    actor,
		Action:   action,
		Entity:   "reconciliation_item",
		Before:   map[string]any{},
		After:    map[string]any{},
		Metadata: metadata,
	})
	return err
}

// AuditJob records a job-level audit event.
func (s *Store) AuditJob(ctx context.Context, jobID int64, action string, metadata map[string]any) error {
	_, err := s.CreateAuditEvent(ctx, &models.AuditEvent{
		Actor:    "system",
		Action:   action,
		Entity:   "reconciliation_job",
		Before:   map[string]any{},
		After:    map[string]any{"job_id": jobID},
		Metadata: metadata,
	})
	return err
}

// --- Human review ----------------------------------------------------------

func (s *Store) CreateHumanReviewAction(ctx context.Context, a *models.HumanReviewAction) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO human_review_actions (item_id, action, actor, note, evidence)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`,
		a.ItemID, a.Action, a.Actor, a.Note, a.Evidence).
		Scan(&id, &a.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create review action: %w", err)
	}
	return id, nil
}

// --- AI decisions ----------------------------------------------------------

func (s *Store) CreateAIDecision(ctx context.Context, d *models.AIDecision) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO ai_decisions (item_id, recommendation, confidence, candidate_rankings, investigation, model_version)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at`,
		d.ItemID, d.Recommendation, d.Confidence, d.CandidateRankings, d.Investigation, d.ModelVersion).
		Scan(&id, &d.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create ai decision: %w", err)
	}
	return id, nil
}

// ListAIDecisionsByItem returns AI recommendations for an item, oldest first.
func (s *Store) ListAIDecisionsByItem(ctx context.Context, itemID int64) ([]*models.AIDecision, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, item_id, recommendation, confidence, candidate_rankings, investigation, model_version, created_at
		FROM ai_decisions WHERE item_id = $1 ORDER BY id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.AIDecision
	for rows.Next() {
		var d models.AIDecision
		if err := rows.Scan(&d.ID, &d.ItemID, &d.Recommendation, &d.Confidence,
			&d.CandidateRankings, &d.Investigation, &d.ModelVersion, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

var _ = pgx.ErrNoRows
