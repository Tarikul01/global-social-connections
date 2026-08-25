#!/usr/bin/env bash
# Apply FindNeighbour schema to Supabase (or any Postgres with PostGIS).
# Usage:
#   export DATABASE_URL='postgresql://postgres:URL_ENCODED_PASSWORD@db.XXXX.supabase.co:5432/postgres?sslmode=require'
#   ./scripts/migrate-supabase.sh
#
# Or: DATABASE_URL=... ./scripts/migrate-supabase.sh

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ -z "${DATABASE_URL:-}" ]]; then
  if [[ -f .env ]]; then
    # shellcheck disable=SC1091
    set -a; source .env; set +a
  fi
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is required (set in env or .env)"
  exit 1
fi

run_psql() {
  if command -v psql >/dev/null 2>&1; then
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 "$@"
  else
    docker run --rm --network=host \
      -v "$ROOT/migrations:/migrations:ro" \
      postgres:16-alpine \
      psql "$DATABASE_URL" -v ON_ERROR_STOP=1 "$@"
  fi
}

echo "==> Enabling PostGIS"
run_psql -c "CREATE EXTENSION IF NOT EXISTS postgis;"

echo "==> 001_initial_schema.sql (tables + PostGIS)"
# When using docker, files are under /migrations; with local psql use relative path
if command -v psql >/dev/null 2>&1; then
  run_psql -f migrations/001_initial_schema.sql
  run_psql -f migrations/003_profile_enrichment.sql
  run_psql -f migrations/004_profile_personal.sql
  run_psql -f migrations/006_event_invitations.sql
else
  run_psql \
    -f /migrations/001_initial_schema.sql \
    -f /migrations/003_profile_enrichment.sql \
    -f /migrations/004_profile_personal.sql \
    -f /migrations/006_event_invitations.sql
fi

echo "==> Done. Listing tables:"
run_psql -c "\dt"

echo ""
echo "Optional demo seed (local/dev only):"
echo "  psql \"\$DATABASE_URL\" -f migrations/002_demo_map_people.sql"
echo "  psql \"\$DATABASE_URL\" -f migrations/005_map_discoverable.sql"
