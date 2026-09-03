package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

const itemCols = `id, job_id, record_id, match_record_id, status, decision, confidence,
	version, created_at, updated_at, resolved_at`

func scanItem(row pgx.Row) (*models.Item, error) {
	var it models.Item
	err := row.Scan(&it.ID, &it.JobID, &it.RecordID, &it.MatchRecordID, &it.Status,
		&it.Decision, &it.Confidence, &it.Version, &it.CreatedAt, &it.UpdatedAt, &it.ResolvedAt)
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// CreateItems bulk-inserts reconciliation items for the given records.
func (s *Store) CreateItems(ctx context.Context, jobID int64, recordIDs []int64) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	n := 0
	for _, rid := range recordIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO reconciliation_items (job_id, record_id, status)
			VALUES ($1, $2, 'PENDING')`, jobID, rid); err != nil {
			return 0, fmt.Errorf("insert item: %w", err)
		}
		n++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return n, nil
}

// GetItem fetches one item.
func (s *Store) GetItem(ctx context.Context, id int64) (*models.Item, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+itemCols+` FROM reconciliation_items WHERE id = $1`, id)
	it, err := scanItem(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	return it, nil
}

// ListItemsByJob returns items for a job, optionally filtered by status, oldest first.
func (s *Store) ListItemsByJob(ctx context.Context, jobID int64, status string, limit, offset int) ([]*models.Item, error) {
	q := `SELECT ` + itemCols + ` FROM reconciliation_items WHERE job_id = $1`
	args := []any{jobID}
	if status != "" {
		q += ` AND status = $2`
		args = append(args, status)
	}
	q += ` ORDER BY id LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()
	var out []*models.Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// CountItemsByStatus counts items for a job in one status (” = all).
func (s *Store) CountItemsByStatus(ctx context.Context, jobID int64, status string) (int64, error) {
	q := `SELECT count(*) FROM reconciliation_items WHERE job_id = $1`
	args := []any{jobID}
	if status != "" {
		q += ` AND status = $2`
		args = append(args, status)
	}
	var n int64
	if err := s.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ListItemsDetailed returns items joined with their record, oldest first.
func (s *Store) ListItemsDetailed(ctx context.Context, jobID int64, status string, limit, offset int) ([]*models.ItemView, error) {
	q := `SELECT i.id, i.job_id, i.record_id, i.match_record_id, i.status, i.decision,
		i.confidence, i.version, i.created_at, i.updated_at, i.resolved_at,
		r.external_id, r.kind, r.amount_minor
	FROM reconciliation_items i JOIN records r ON r.id = i.record_id
	WHERE i.job_id = $1`
	args := []any{jobID}
	if status != "" {
		q += ` AND i.status = $2`
		args = append(args, status)
	}
	q += ` ORDER BY i.id LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list items detailed: %w", err)
	}
	defer rows.Close()
	var out []*models.ItemView
	for rows.Next() {
		var v models.ItemView
		if err := rows.Scan(&v.ID, &v.JobID, &v.RecordID, &v.MatchRecordID, &v.Status, &v.Decision,
			&v.Confidence, &v.Version, &v.CreatedAt, &v.UpdatedAt, &v.ResolvedAt,
			&v.RecordExternalID, &v.RecordKind, &v.AmountMinor); err != nil {
			return nil, err
		}
		out = append(out, &v)
	}
	return out, rows.Err()
}

// CountActiveItems counts items for a job still in a non-terminal state.
func (s *Store) CountActiveItems(ctx context.Context, jobID int64) (int64, error) {
	var n int64
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM reconciliation_items
		WHERE job_id = $1 AND status IN ('PENDING','MATCHING','VERIFYING','INVESTIGATING')`,
		jobID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ClaimItem claims the next PENDING item of a RUNNING job for processing,
// atomically moving it to MATCHING. Returns ErrNotFound when nothing is queued.
func (s *Store) ClaimItem(ctx context.Context) (*models.Item, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		SELECT i.id FROM reconciliation_items i
		JOIN reconciliation_jobs j ON j.id = i.job_id
		WHERE j.status = 'RUNNING' AND i.status = 'PENDING'
		ORDER BY i.id
		FOR UPDATE SKIP LOCKED
		LIMIT 1`)
	var id int64
	if err := row.Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("claim: %w", err)
	}

	var it models.Item
	if err := tx.QueryRow(ctx, `SELECT `+itemCols+` FROM reconciliation_items WHERE id = $1 FOR UPDATE`, id).
		Scan(&it.ID, &it.JobID, &it.RecordID, &it.MatchRecordID, &it.Status,
			&it.Decision, &it.Confidence, &it.Version, &it.CreatedAt, &it.UpdatedAt, &it.ResolvedAt); err != nil {
		return nil, fmt.Errorf("claim load: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE reconciliation_items SET status = 'MATCHING', updated_at = now(), version = version + 1
		WHERE id = $1 AND version = $2`, it.ID, it.Version); err != nil {
		return nil, fmt.Errorf("claim mark: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("claim commit: %w", err)
	}
	it.Status = models.StatusMatching
	it.Version++
	return &it, nil
}

// UpdateItem persists an item with optimistic locking (item.Version is the
// version read by the caller). Returns ErrStaleVersion on concurrent change.
func (s *Store) UpdateItem(ctx context.Context, it *models.Item) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE reconciliation_items SET
			status = $2,
			match_record_id = $3,
			decision = $4,
			confidence = $5,
			updated_at = now(),
			resolved_at = $6,
			version = version + 1
		WHERE id = $1 AND version = $7`,
		it.ID, it.Status, it.MatchRecordID, it.Decision, it.Confidence, it.ResolvedAt, it.Version)
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleVersion
	}
	it.Version++
	return nil
}
