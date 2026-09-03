package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

const jobCols = `id, name, status, config, total_records, matched, review, escalated,
	version, created_at, updated_at, completed_at`

func scanJob(row pgx.Row) (*models.Job, error) {
	var j models.Job
	err := row.Scan(&j.ID, &j.Name, &j.Status, &j.Config, &j.TotalRecords,
		&j.Matched, &j.Review, &j.Escalated, &j.Version, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// CreateJob inserts a new reconciliation job in PENDING state.
func (s *Store) CreateJob(ctx context.Context, name string, config map[string]any) (*models.Job, error) {
	if config == nil {
		config = map[string]any{}
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO reconciliation_jobs (name, config, status)
		VALUES ($1, $2, 'PENDING') RETURNING `+jobCols, name, config)
	j, err := scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	return j, nil
}

// GetJob fetches a job by id.
func (s *Store) GetJob(ctx context.Context, id int64) (*models.Job, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+jobCols+` FROM reconciliation_jobs WHERE id = $1`, id)
	j, err := scanJob(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return j, nil
}

// ListJobs returns the most recent jobs first.
func (s *Store) ListJobs(ctx context.Context, limit, offset int) ([]*models.Job, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+jobCols+` FROM reconciliation_jobs ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()
	var out []*models.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// SetJobStatus transitions a job. Returns ErrStaleVersion on a concurrent change.
func (s *Store) SetJobStatus(ctx context.Context, id, wantVersion int64, status string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE reconciliation_jobs SET status = $1, updated_at = now(), version = version + 1
		WHERE id = $2 AND version = $3`, status, id, wantVersion)
	if err != nil {
		return fmt.Errorf("set job status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleVersion
	}
	return nil
}

// CompleteJob marks a job COMPLETED with its completion time.
func (s *Store) CompleteJob(ctx context.Context, id, wantVersion int64) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE reconciliation_jobs SET status = 'COMPLETED', completed_at = now(),
			updated_at = now(), version = version + 1
		WHERE id = $1 AND version = $2`, id, wantVersion)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleVersion
	}
	return nil
}

// SetJobTotal records the number of items created for a job. It is written
// once, right after the job is marked RUNNING and before any worker can finish
// the job, so a plain UPDATE by id is safe here.
func (s *Store) SetJobTotal(ctx context.Context, id, total int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE reconciliation_jobs SET total_records = $1, updated_at = now()
		WHERE id = $2`, total, id)
	if err != nil {
		return fmt.Errorf("set job total: %w", err)
	}
	return nil
}

// ListRunningJobs returns all jobs still being processed.
func (s *Store) ListRunningJobs(ctx context.Context) ([]*models.Job, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+jobCols+` FROM reconciliation_jobs WHERE status = 'RUNNING' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list running jobs: %w", err)
	}
	defer rows.Close()
	var out []*models.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// IncJobStat increments one of the counted job stats by delta.
func (s *Store) IncJobStat(ctx context.Context, id int64, column string, delta int64) error {
	q := fmt.Sprintf(`UPDATE reconciliation_jobs SET %s = %s + $1, updated_at = now() WHERE id = $2`, column, column)
	_, err := s.pool.Exec(ctx, q, delta, id)
	return err
}
