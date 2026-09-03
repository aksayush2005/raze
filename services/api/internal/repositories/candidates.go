package repositories

import (
	"context"
	"fmt"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

// PaymentsForSettlement returns all payments that reference the settlement by
// their ref_external_id (the N:1 batch relationship).
func (s *Store) PaymentsForSettlement(ctx context.Context, settlementExternalID string) ([]*models.Record, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+recordCols+` FROM records
		WHERE kind = 'payment' AND ref_external_id = $1
		ORDER BY id`, settlementExternalID)
	if err != nil {
		return nil, fmt.Errorf("payments for settlement: %w", err)
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

// SettlementByExternalID fetches a settlement record by external_id (used to
// resolve a payment's ref_external_id into its settlement).
func (s *Store) SettlementByExternalID(ctx context.Context, externalID string) (*models.Record, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+recordCols+` FROM records WHERE kind = 'settlement' AND external_id = $1`, externalID)
	return scanRecord(row)
}

// FuzzyCandidates returns candidate records of the opposite kind within the
// same currency, amount tolerance and time window — the deterministic fuzzy
// signal used when no exact reference exists.
func (s *Store) FuzzyCandidates(ctx context.Context, rec *models.Record, amountTolMinor int64, windowHours int, limit int) ([]*models.Record, error) {
	opposite := "payment"
	if rec.Kind == "payment" {
		opposite = "settlement"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+recordCols+` FROM records
		WHERE kind = $1 AND currency = $2
		  AND amount_minor BETWEEN $3::bigint - $4::bigint AND $3::bigint + $4::bigint
		  AND occurred_at BETWEEN $5::timestamptz - make_interval(hours => $6::int) AND $5::timestamptz + make_interval(hours => $6::int)
		  AND id <> $7
		ORDER BY amount_minor, occurred_at
		LIMIT $8`,
		opposite, rec.Currency, rec.AmountMinor, amountTolMinor, rec.OccurredAt, windowHours, rec.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("fuzzy candidates: %w", err)
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
