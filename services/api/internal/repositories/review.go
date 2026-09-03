package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

// ApplyReview atomically records a human review action, transitions the item,
// and appends an audit event. Returns ErrStaleVersion on concurrent change.
func (s *Store) ApplyReview(ctx context.Context, a *models.HumanReviewAction, it *models.Item) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin review: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO human_review_actions (item_id, action, actor, note, evidence)
		VALUES ($1,$2,$3,$4,$5)`, a.ItemID, a.Action, a.Actor, a.Note, a.Evidence); err != nil {
		return fmt.Errorf("review insert action: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE reconciliation_items SET
			status = $2, decision = $3, match_record_id = $4, confidence = $5,
			updated_at = now(), resolved_at = $6, version = version + 1
		WHERE id = $1 AND version = $7`,
		it.ID, it.Status, it.Decision, it.MatchRecordID, it.Confidence, it.ResolvedAt, it.Version)
	if err != nil {
		return fmt.Errorf("review update item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleVersion
	}

	itemID := it.ID
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_events (item_id, actor, action, entity, before, after, metadata)
		VALUES ($1,$2,$3,'reconciliation_item','{}','{}',$4)`,
		itemID, "human:"+a.Actor, a.Action, map[string]any{
			"decision": it.Decision,
			"note":     a.Note,
		}); err != nil {
		return fmt.Errorf("review audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("review commit: %w", err)
	}
	it.Version++
	return nil
}

var _ = pgx.ErrNoRows
