#!/bin/sh
set -e
cd /app

# Do NOT shell-source .env — passwords contain *, %, & which break `set -a; . file`.
# Go loads /app/.env via godotenv (literal values).

if [ -n "${PORT:-}" ]; then
  export APP_PORT="$PORT"
fi
export APP_PORT="${APP_PORT:-80}"
export APP_ENV="${APP_ENV:-production}"

exec /app/server
