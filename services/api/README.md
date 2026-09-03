# RAZE API service

Go control plane: authoritative financial state, reconciliation engine,
human review, audit, idempotency, concurrency control.

## Layout

- `cmd/api`     — HTTP API server
- `cmd/migrate` — database migration runner (goose)
- `internal/`   — application code
  - `handlers/`    — HTTP handlers
  - `services/`    — orchestration / workflow logic
  - `repositories/`— database access
  - `models/`      — domain types
  - `money`        — integer minor-unit money
