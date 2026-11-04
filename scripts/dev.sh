#!/usr/bin/env bash
#
# Start the local GoEasyERP development environment (backend + frontend).
#
#   ./scripts/dev.sh
#
# On first run it bootstraps a SQLite database, starts the Go backend on :8080
# and the SvelteKit dev server on :5173, and creates an initial admin user.
# Press Ctrl+C to stop both servers.
#
# Overridable via environment variables:
#   OPENERP_DB        SQLite database file            (default: dev.db)
#   OPENERP_COMPANY   company to enter                (default: cronus)
#   OPENERP_USER      initial admin user id           (default: ADMIN)
#   OPENERP_PASSWORD  initial admin password (>=6)    (default: admin123)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DB="${OPENERP_DB:-dev.db}"
COMPANY="${OPENERP_COMPANY:-cronus}"
ADMIN_USER="${OPENERP_USER:-ADMIN}"
ADMIN_PASS="${OPENERP_PASSWORD:-admin123}"

# ---------------------------------------------------------------------------
# 1. Bootstrap the SQLite database on first run.
#
# The backend's OpenDatabase() expects an existing DB with a Company table and
# the company row already present (it does not create them in SQLite mode).
# Everything else — the Menu table, per-company tables, seed data — is created by
# the backend's migrations and table sync on startup.
# ---------------------------------------------------------------------------
if [ ! -s "$DB" ]; then
  echo "→ Bootstrapping database '$DB' (company '$COMPANY')..."
  sqlite3 "$DB" <<SQL
CREATE TABLE IF NOT EXISTS "Company" (
    name TEXT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS "User" (
    username TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    full_name TEXT,
    language TEXT DEFAULT 'en-US',
    active INTEGER DEFAULT 1
);
INSERT OR IGNORE INTO "Company" (name) VALUES ('$COMPANY');
SQL
fi

# ---------------------------------------------------------------------------
# 2. Clean up child processes on exit.
# ---------------------------------------------------------------------------
BACKEND_PID=""
FRONTEND_PID=""
cleanup() {
  echo
  echo "→ Stopping servers..."
  [ -n "$FRONTEND_PID" ] && kill "$FRONTEND_PID" 2>/dev/null || true
  [ -n "$BACKEND_PID" ]  && kill "$BACKEND_PID"  2>/dev/null || true
}
trap cleanup INT TERM EXIT

# ---------------------------------------------------------------------------
# 3. Start the backend (feeds the DB path and company to its stdin prompts).
# ---------------------------------------------------------------------------
echo "→ Starting backend on http://localhost:8080 ..."
printf '%s\n%s\n' "$DB" "$COMPANY" | go run ./cmd/api-server &
BACKEND_PID=$!

echo "→ Waiting for backend to be ready..."
for _ in $(seq 1 60); do
  if curl -sf -o /dev/null http://localhost:8080/health 2>/dev/null; then
    echo "✓ Backend ready."
    break
  fi
  if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    echo "✗ Backend exited during startup. See output above." >&2
    exit 1
  fi
  sleep 1
done

# ---------------------------------------------------------------------------
# 4. Create the initial admin user (idempotent — 403 means it already exists).
# ---------------------------------------------------------------------------
init_status=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:8080/api/auth/init \
  -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"$ADMIN_USER\",\"user_name\":\"Administrator\",\"email\":\"admin@example.com\",\"password\":\"$ADMIN_PASS\"}" 2>/dev/null || true)
case "$init_status" in
  200) echo "✓ Created initial user '$ADMIN_USER' (password '$ADMIN_PASS')." ;;
  403) echo "✓ User already exists — skipping initial-user creation." ;;
  *)   echo "! Initial-user request returned HTTP $init_status (continuing)." ;;
esac

# ---------------------------------------------------------------------------
# 5. Start the frontend dev server (proxies /api to the backend).
# ---------------------------------------------------------------------------
echo "→ Starting frontend on http://localhost:5173 ..."
( cd frontend && npm run dev ) &
FRONTEND_PID=$!

echo
echo "════════════════════════════════════════════════════════"
echo "  GoEasyERP dev environment is starting up."
echo "  App:      http://localhost:5173"
echo "  API:      http://localhost:8080"
echo "  Login:    $ADMIN_USER / $ADMIN_PASS   (company: $COMPANY)"
echo "  Stop:     press Ctrl+C"
echo "════════════════════════════════════════════════════════"
echo

# Wait for either server to exit, then cleanup runs via the trap.
wait
