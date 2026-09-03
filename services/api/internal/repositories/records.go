package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

const recordCols = `id, source, is_synthetic, external_id, kind, status,
	amount_minor, fee_minor, tax_minor, net_minor, currency, occurred_at,
	ref_external_id, truth_group, created_at, updated_at`

func scanRecord(row pgx.Row) (*models.Record, error) {
	var r models.Record
	err := row.Scan(
		&r.ID, &r.Source, &r.IsSynthetic, &r.ExternalID, &r.Kind, &r.Status,
		&r.AmountMinor, &r.FeeMinor, &r.TaxMinor, &r.NetMinor, &r.Currency, &r.OccurredAt,
		&r.RefExternalID, &r.TruthGroup, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UpsertRecord inserts a record or updates it in place when (source, external_id)
// already exists. Returns the record's id.
func (s *Store) UpsertRecord(ctx context.Context, r *models.Record) (int64, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO records (source, is_synthetic, external_id, kind, status,
			amount_minor, fee_minor, tax_minor, net_minor, currency, occurred_at,
			ref_external_id, truth_group)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (source, external_id) DO UPDATE SET
			kind = EXCLUDED.kind,
			status = EXCLUDED.status,
			amount_minor = EXCLUDED.amount_minor,
			fee_minor = EXCLUDED.fee_minor,
			tax_minor = EXCLUDED.tax_minor,
			net_minor = EXCLUDED.net_minor,
			currency = EXCLUDED.currency,
			occurred_at = EXCLUDED.occurred_at,
			ref_external_id = EXCLUDED.ref_external_id,
			updated_at = now()
		RETURNING id`,
		r.Source, r.IsSynthetic, r.ExternalID, r.Kind, r.Status,
		r.AmountMinor, r.FeeMinor, r.TaxMinor, r.NetMinor, r.Currency, r.OccurredAt,
		r.RefExternalID, r.TruthGroup,
	)
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, fmt.Errorf("upsert record: %w", err)
	}
	return id, nil
}

// GetRecord fetches one record by id.
func (s *Store) GetRecord(ctx context.Context, id int64) (*models.Record, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+recordCols+` FROM records WHERE id = $1`, id)
	r, err := scanRecord(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get record: %w", err)
	}
	return r, nil
}

// AllRecordIDs returns the ids of every active record, oldest first.
func (s *Store) AllRecordIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM records ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("all record ids: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ListRecords returns records optionally filtered by kind, newest first, paginated.
func (s *Store) ListRecords(ctx context.Context, kind string, limit, offset int) ([]*models.Record, error) {
	q := `SELECT ` + recordCols + ` FROM records`
	args := []any{limit, offset}
	if kind != "" {
		q += ` WHERE kind = $1`
		args = []any{kind, limit, offset}
	}
	q += ` ORDER BY id DESC LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}
	defer rows.Close()
	var out []*models.Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
