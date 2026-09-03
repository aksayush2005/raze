#!/usr/bin/env bash
# Full RAZE end-to-end demo.
#  1. start PostgreSQL (docker)
#  2. apply migrations
#  3. generate synthetic records with known ground truth
#  4. start the Go control plane (and the Python AI service, if available)
#  5. import records, run a reconciliation, then show a reviewable item
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$(pwd)"

# Load the gitignored .env if present so RAZORPAY_KEY_ID/SECRET reach the
# host-run API (docker compose reads .env itself; these scripts don't).
if [ -f "$ROOT/.env" ]; then
  set -a
  # shellcheck disable=SC1091
  . "$ROOT/.env"
  set +a
fi

API_PORT="${API_PORT:-8080}"
AI_URL="${AI_SERVICE_URL:-http://localhost:8090}"
BASE="http://localhost:${API_PORT}"

echo "==> [1/6] PostgreSQL"
docker compose up -d postgres
until docker compose exec -T postgres pg_isready -U raze -d raze >/dev/null 2>&1; do sleep 1; done

export DATABASE_URL="${DATABASE_URL:-postgres://raze:raze_dev_password@localhost:5432/raze?sslmode=disable}"

echo "==> [2/6] Migrations"
cd services/api && go run ./cmd/migrate && cd "$ROOT"

echo "==> [3/6] Synthetic data"
python3 data/generator/generate.py --n-settlements 150 --corruption-rate 0.30 --seed 42

echo "==> [4/6] Services"
(cd services/api && go build -o /tmp/raze-api ./cmd/api)
/tmp/raze-api &
API_PID=$!
trap 'kill ${API_PID:-} ${AI_PID:-} 2>/dev/null || true' EXIT
until curl -sf "$BASE/healthz" >/dev/null 2>&1; do sleep 0.5; done
echo "    api up on $BASE"

if command -v python3 >/dev/null 2>&1 && python3 -c 'import fastapi,uvicorn' 2>/dev/null; then
  (cd ai && exec python3 -m uvicorn app.main:app --port 8090) >/tmp/raze_ai.log 2>&1 &
  AI_PID=$!
  echo "    ai service up on $AI_URL"
else
  echo "    ai service NOT started (fastapi/uvicorn missing) — control plane runs deterministic-only"
fi

echo "==> [5/6] Reconcile"
curl -sf -X POST "$BASE/api/v1/records/import" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-import-1' \
  -d @"$ROOT/data/benchmark/records.json" | python3 -m json.tool

JOB_ID=$(curl -sf -X POST "$BASE/api/v1/jobs" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-job-1' \
  -d '{"name":"demo-run","config":{"corruption_rate":0.30}}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')
echo "    job $JOB_ID created"

for i in $(seq 1 60); do
  STATUS=$(curl -sf "$BASE/api/v1/jobs/$JOB_ID" | python3 -c 'import sys,json;print(json.load(sys.stdin)["status"])')
  [ "$STATUS" = "COMPLETED" ] && break
  sleep 1
done
echo "==> [6/6] Result"
curl -sf "$BASE/api/v1/jobs/$JOB_ID" | python3 -m json.tool

echo
echo "Job finished. Open an item to review:"
echo "    GET $BASE/api/v1/items/1"
echo "    POST $BASE/api/v1/items/{id}/review  {\"action\":\"ACCEPTED_AGENT_MATCH\",\"actor\":\"operator@demo\"}"
