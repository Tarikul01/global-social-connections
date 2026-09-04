#!/bin/sh
set -e

# Force-load selected keys from baked /app/.env (from env.vercel).
# Vercel dashboard env wins over godotenv.Load — a bad DATABASE_URL there
# crashes the process with FUNCTION_INVOCATION_FAILED. We always prefer the
# image values for DB/secrets. Never override PORT (platform listen port).
ENV_FILE=/app/.env
load_baked_key() {
	key="$1"
	[ -f "$ENV_FILE" ] || return 0
	line=$(grep -m1 "^${key}=" "$ENV_FILE" || true)
	[ -n "$line" ] || return 0
	val=${line#*=}
	# Strip optional surrounding quotes
	case "$val" in
		\"*\") val=${val#\"}; val=${val%\"} ;;
		\'*\') val=${val#\'}; val=${val%\'} ;;
	esac
	export "$key=$val"
}

if [ -f "$ENV_FILE" ]; then
	load_baked_key DATABASE_URL
	load_baked_key APP_SECRET
	load_baked_key CSRF_SECRET
	load_baked_key APP_ENV
	load_baked_key APP_URL
	load_baked_key FRONTEND_URL
	load_baked_key SUPABASE_URL
	load_baked_key SUPABASE_SERVICE_KEY
	load_baked_key SESSION_DURATION_HOURS
	load_baked_key ARGON2_MEMORY
	load_baked_key ARGON2_ITERATIONS
	load_baked_key ARGON2_PARALLELISM
	load_baked_key SMTP_HOST
	load_baked_key SMTP_PORT
	load_baked_key SMTP_USER
	load_baked_key SMTP_PASS
	load_baked_key SMTP_FROM
fi

# Platform PORT always wins for listen address.
if [ -n "${PORT:-}" ]; then
	export APP_PORT="$PORT"
fi
export APP_PORT="${APP_PORT:-80}"
export APP_ENV="${APP_ENV:-production}"

exec /app/server
