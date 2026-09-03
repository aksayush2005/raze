// Package repositories provides database access for the control plane.
// All financial mutations happen through these queries; nothing else writes.
package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors.
var (
	ErrNotFound     = errors.New("not found")
	ErrStaleVersion = errors.New("stale version: concurrent update")
)

// Store is the single database access wrapper.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store backed by the given connection pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the underlying pool (used by the worker for claim transactions).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func (s *Store) Close() { s.pool.Close() }

var _ = context.Background
