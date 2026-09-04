#!/bin/sh
set -e
# Vercel injects PORT; map it to APP_PORT for the Go server.
if [ -n "${PORT:-}" ]; then
  export APP_PORT="$PORT"
fi
export APP_PORT="${APP_PORT:-80}"
exec /app/server
