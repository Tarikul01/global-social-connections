# Deploying FindNeighbour on Vercel (Docker)

Vercel-only files are named `*.vercel*` / `vercel.json` so local Docker stays untouched.

> **Postgres is never on Vercel.** Use external PostGIS and set `DATABASE_URL`.

## Vercel-only file map

| File | Purpose |
|------|---------|
| `vercel.json` | Multi-service containers + `/api` rewrites |
| `backend/Dockerfile.vercel` | Go API image for Vercel (`$PORT`) |
| `backend/docker-entrypoint.vercel.sh` | Maps `PORT` → Go listen port |
| `backend/.env.vercel.example` | Env template → copy to `backend/.env.vercel` |
| `frontend/Dockerfile.vercel` | SPA nginx image for Vercel |
| `frontend/docker-entrypoint.vercel.sh` | nginx listen on `$PORT` |
| `frontend/nginx.vercel.conf` | SPA-only nginx (no API proxy) |
| `.env.vercel.example` | Dashboard env checklist |
| `docker-compose.vercel.yml` | **Local test** of Vercel images + rewrite gateway |
| `nginx.vercel.gateway.conf` | Gateway used by compose.vercel (mimics `vercel.json`) |
| `DEPLOY.md` | This guide |

**Local (unchanged):** `docker-compose.yml`, `backend/Dockerfile`, `frontend/Dockerfile`, `backend/.env.example`, `frontend/nginx.conf`.

## Architecture on Vercel

```
Browser  →  https://global-social-connections.vercel.app
              ├─ /api/* , /healthz  →  backend (Dockerfile.vercel)
              └─ /*                →  frontend (Dockerfile.vercel)
```

## 1. Env files

```bash
cp -n backend/.env.vercel.example backend/.env.vercel
# Edit APP_SECRET and DATABASE_URL (external PostGIS for real Vercel deploy)
```

Paste the same keys into **Vercel → Settings → Environment Variables** (see `.env.vercel.example`).

## 2. Database

Apply migrations against your external PostGIS DB:

- `backend/migrations/001_initial_schema.sql`
- `backend/migrations/003_profile_enrichment.sql`
- `backend/migrations/004_profile_personal.sql`
- `backend/migrations/006_event_invitations.sql`

## 3. Test Vercel images locally (optional)

```bash
cp -n backend/.env.vercel.example backend/.env.vercel
# set APP_SECRET (compose overrides DATABASE_URL to bundled postgres)

docker compose -f docker-compose.vercel.yml up --build
```

Open **http://localhost:3080** — gateway routing matches `vercel.json`.

| Service  | Port |
|----------|------|
| Gateway (SPA + API) | 3080 |
| PostGIS (host) | 5434 |

## 4. Deploy to Vercel

```bash
# From repo root
vercel link
vercel --prod
```

Required dashboard env: `DATABASE_URL`, `APP_SECRET`, `APP_ENV=production`,
`APP_URL=https://global-social-connections.vercel.app`,
`FRONTEND_URL=https://global-social-connections.vercel.app`.

Enable **Fluid compute** (needed for WebSockets).

## 5. Verify

- https://global-social-connections.vercel.app/healthz
- Map: https://global-social-connections.vercel.app/ → one WebSocket to `/api/v1/ws`
- Login cookies work same-origin

## Limits

- WebSocket pinned to one instance + max duration; in-memory presence does not sync across scaled instances.
- Container disk is ephemeral — use Supabase Storage for durable photos.

## Local Docker (non-Vercel)

```bash
cp -n backend/.env.example backend/.env
docker compose up --build
# http://localhost:3000
```
