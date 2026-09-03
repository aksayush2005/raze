package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrIdempotencyConflict indicates the same key was reused with a different request.
var ErrIdempotencyConflict = errors.New("idempotency key reused with different request")

// GetIdempotency returns the stored response and its request hash for a key.
func (s *Store) GetIdempotency(ctx context.Context, key string) (resp []byte, hash string, found bool, err error) {
	var raw json.RawMessage
	err = s.pool.QueryRow(ctx, `
		SELECT response, request_hash FROM idempotency_keys
		WHERE key = $1 AND expires_at > now()`, key).Scan(&raw, &hash)
	if err == pgx.ErrNoRows {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, fmt.Errorf("get idempotency: %w", err)
	}
	return raw, hash, true, nil
}

// SetIdempotency stores a response under a key only if the key is new or the
// request hash matches the stored one. Returns ErrIdempotencyConflict otherwise.
// Reclaiming an existing key (e.g. one whose window has expired) refreshes its
// expiry so a reused key behaves as a fresh one rather than being permanently
// stuck in the past.
func (s *Store) SetIdempotency(ctx context.Context, key, requestHash string, response []byte) error {
	var storedHash string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO idempotency_keys (key, request_hash, response)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET response = EXCLUDED.response,
			expires_at = now() + interval '24 hours'
		WHERE idempotency_keys.request_hash = EXCLUDED.request_hash
		RETURNING request_hash`, key, requestHash, response).Scan(&storedHash)
	if err == pgx.ErrNoRows {
		return ErrIdempotencyConflict
	}
	if err != nil {
		return fmt.Errorf("set idempotency: %w", err)
	}
	return nil
}
