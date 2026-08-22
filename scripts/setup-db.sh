#!/usr/bin/env bash
# Create / reset the FindNeighbour database on THIS project's Postgres.
# Usage: ./scripts/setup-db.sh
#
# Expects docker compose postgres (gsc-postgres-1) on port 5433.

set -euo pipefail

CONTAINER="${1:-gsc-postgres-1}"
PGUSER="${PGUSER:-findneighbour}"
PGPASSWORD="${PGPASSWORD:-findneighbour}"
DB_NAME="${DB_NAME:-findneighbour}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATION="${SCRIPT_DIR}/../migrations/001_initial_schema.sql"

echo "==> Using container: ${CONTAINER}"
echo "==> Database name: ${DB_NAME}"

if ! docker ps --format '{{.Names}}' | grep -qx "${CONTAINER}"; then
  echo "Container ${CONTAINER} is not running. Start it with:"
  echo "  docker compose up -d"
  exit 1
fi

echo "==> Recreating database ${DB_NAME}..."
docker exec -e PGPASSWORD="${PGPASSWORD}" "${CONTAINER}" \
  psql -U "${PGUSER}" -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='${DB_NAME}' AND pid <> pg_backend_pid();" >/dev/null || true
docker exec -e PGPASSWORD="${PGPASSWORD}" "${CONTAINER}" \
  psql -U "${PGUSER}" -d postgres -c "DROP DATABASE IF EXISTS ${DB_NAME};"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${CONTAINER}" \
  psql -U "${PGUSER}" -d postgres -c "CREATE DATABASE ${DB_NAME};"

echo "==> Running migration..."
docker exec -i -e PGPASSWORD="${PGPASSWORD}" "${CONTAINER}" \
  psql -U "${PGUSER}" -d "${DB_NAME}" < "${MIGRATION}"

echo "==> Done!"
echo "DATABASE_URL=postgresql://${PGUSER}:${PGPASSWORD}@localhost:5433/${DB_NAME}?sslmode=disable"
