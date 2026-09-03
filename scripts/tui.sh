#!/usr/bin/env bash
# RAZE terminal UI launcher.
# Boots the stack if needed (postgres, migrations, synthetic data, Python AI
# service, Go API), then drops you into the interactive TUI. Both services stay
# up only while the TUI is running and are torn down on exit.
#
# Usage: bash scripts/tui.sh [raze-tui flags...]   e.g. --actor operator@ops
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$(pwd)"

# Load the gitignored .env if present so RAZORPAY_KEY_ID/SECRET, GEMINI_API_KEY
# etc. reach the host-run binaries (docker compose reads .env itself; these
# scripts don't).
if [ -f "$ROOT/.env" ]; then
  set -a
  # shellcheck disable=SC1091
  . "$ROOT/.env"
  set +a
fi

# podman under the VS Code snap inherits a snap-scoped XDG_DATA_HOME that breaks
# container storage. Redirect it to the real default when that happens.
if [[ "${XDG_DATA_HOME:-}" == *"/snap/code/"* ]]; then
  export XDG_DATA_HOME="$HOME/.local/share"
  echo "    (podman workaround: XDG_DATA_HOME -> $XDG_DATA_HOME)"
fi

API_PORT="${API_PORT:-8080}"
AI_PORT="${AI_PORT:-8090}"
AI_URL="http://127.0.0.1:${AI_PORT}"
AI_DIR="$ROOT/ai"
BASE="http://localhost:${API_PORT}"
export DATABASE_URL="${DATABASE_URL:-postgres://raze:raze_dev_password@localhost:5432/raze?sslmode=disable}"

echo "==> [1/7] PostgreSQL"
docker compose up -d postgres
until docker compose exec -T postgres pg_isready -U raze -d raze >/dev/null 2>&1; do sleep 1; done

echo "==> [2/7] Migrations"
(cd services/api && go run ./cmd/migrate)

echo "==> [3/7] Synthetic data"
if [ ! -f "$ROOT/data/benchmark/records.json" ]; then
  python3 data/generator/generate.py --n-settlements 150 --corruption-rate 0.30 --seed 42
else
  echo "    reusing existing data/benchmark/records.json"
fi

# --- Python AI advisory service (optional but recommended) -------------------
# Brings up the investigation service in an ai/.venv. Never breaks `make tui`:
# if the venv, dependencies or network are unavailable it degrades to
# deterministic-only (the API just logs ai=false). Respects an AI service that
# is already running on AI_PORT.
start_ai() {
  echo "==> [4/7] AI service"
  if curl -sf "$AI_URL/healthz" >/dev/null 2>&1; then
    echo "    ai already up on $AI_URL"
    return 0
  fi
  local PY="$AI_DIR/.venv/bin/python"
  if [ ! -x "$PY" ]; then
    echo "    creating ai/.venv"
    # Some Debian/Ubuntu python3 builds lack ensurepip, so a bare `venv` fails.
    # Fall back to --without-pip and bootstrap pip from get-pip.py (needs network).
    if ! python3 -m venv "$AI_DIR/.venv" 2>/dev/null; then
      rm -rf "$AI_DIR/.venv"
      if ! python3 -m venv --without-pip "$AI_DIR/.venv" 2>/dev/null; then
        echo "    AI service disabled: python3 venv unavailable — deterministic only"
        return 1
      fi
      if ! curl -fsSL -m 60 https://bootstrap.pypa.io/get-pip.py -o /tmp/raze_get_pip.py \
          || ! "$PY" /tmp/raze_get_pip.py --quiet; then
        echo "    AI service disabled: cannot bootstrap pip (offline?) — deterministic only"
        return 1
      fi
      rm -f /tmp/raze_get_pip.py
    fi
  fi
  if ! "$PY" -c 'import fastapi, uvicorn, google.genai' >/dev/null 2>&1; then
    echo "    installing ai dependencies (needs network)"
    if ! "$PY" -m pip install --quiet -r "$AI_DIR/requirements.txt"; then
      echo "    AI service disabled: pip install failed (offline?) — deterministic only"
      return 1
    fi
  fi
  (cd "$AI_DIR" && exec "$PY" -m uvicorn app.main:app --host 127.0.0.1 --port "$AI_PORT") \
    >/tmp/raze_ai.log 2>&1 &
  AI_PID=$!
  local i
  for i in $(seq 1 30); do
    if curl -sf "$AI_URL/healthz" >/dev/null 2>&1; then
      echo "    ai up on $AI_URL"
      return 0
    fi
    sleep 0.5
  done
  echo "    AI service failed to start (see /tmp/raze_ai.log) — deterministic only"
  kill "$AI_PID" 2>/dev/null || true
  return 1
}
AI_PID=""
start_ai || true
# Tear down whatever services this script started when it exits.
trap 'kill ${API_PID:-} ${AI_PID:-} 2>/dev/null || true' EXIT

echo "==> [5/7] Build"
(cd services/api && go build -o /tmp/raze-tui ./cmd/raze-tui)
(cd services/api && go build -o /tmp/raze-api ./cmd/api)

echo "==> [6/7] API"
/tmp/raze-api &
API_PID=$!
until curl -sf "$BASE/healthz" >/dev/null 2>&1; do sleep 0.5; done
echo "    api up on $BASE"

# Seed data and run a reconciliation only when the control plane is empty.
if curl -sf "$BASE/api/v1/jobs?limit=1" | python3 -c 'import sys,json; sys.exit(0 if json.load(sys.stdin).get("jobs") else 1)'; then
  echo "    existing data; skipping seed"
else
  echo "    first-run seed (import + reconcile)"
  curl -sf -X POST "$BASE/api/v1/records/import" \
    -H 'Content-Type: application/json' -H 'Idempotency-Key: tui-boot-import' \
    -d @"$ROOT/data/benchmark/records.json" | python3 -m json.tool
  JOB_ID=$(curl -sf -X POST "$BASE/api/v1/jobs" \
    -H 'Content-Type: application/json' -H 'Idempotency-Key: tui-boot-job' \
    -d '{"name":"reconciliation","config":{}}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')
  echo "    job $JOB_ID created"
fi

echo "==> [7/7] Terminal UI"
echo "    exit with q"
echo
/tmp/raze-tui --api "$BASE" "$@"
rc=$?
echo
echo "    services stopped"
exit $rc
