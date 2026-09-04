#!/bin/sh
set -e
cd /app

# Load baked production env (from env.vercel → /app/.env at image build).
# Shell-exported vars are available to the Go process; Vercel can still override
# individual keys if set in the dashboard (exported after this block would win —
# we apply PORT last so Fluid always binds correctly).
if [ -f /app/.env ]; then
  set -a
  # shellcheck disable=SC1091
  . /app/.env
  set +a
fi

if [ -n "${PORT:-}" ]; then
  export APP_PORT="$PORT"
fi
export APP_PORT="${APP_PORT:-80}"
export APP_ENV="${APP_ENV:-production}"

exec /app/server
