# Deploying FindNeighbour on Vercel (Docker)

Vercel-only files are named `*.vercel*` / `vercel.json` so local Docker stays untouched.

> **Postgres is never on Vercel.** Use external PostGIS and set `DATABASE_URL`.

## Vercel-only file map

| File | Purpose |
|------|---------|
| `vercel.json` | Multi-service containers + `/api` rewrites |
| `backend/env.vercel` | Production env baked into the backend image |
| `backend/Dockerfile.vercel` | Go API image for Vercel (`$PORT`) |
| `backend/docker-entrypoint.vercel.sh` | Loads `/app/.env`, maps `PORT` |
| `backend/.env.vercel.example` | Optional local compose template only |
| `frontend/Dockerfile.vercel` | SPA nginx image for Vercel |
| `frontend/docker-entrypoint.vercel.sh` | nginx listen on `$PORT` |
| `frontend/nginx.vercel.conf` | SPA-only nginx (no API proxy) |
| `.env.vercel.example` | Dashboard env checklist |
| `docker-compose.vercel.yml` | **Local test** of Vercel images + rewrite gateway |
| `nginx.vercel.gateway.conf` | Gateway used by compose.vercel (mimics `vercel.json`) |
| `DEPLOY.md` | This guide |

**Local (unchanged):** `docker-compose.yml`, `backend/Dockerfile`, `frontend/Dockerfile`, `backend/.env.example`, `frontend/nginx.conf`.

## Production env on Vercel (baked into Docker)

Production settings live in **`backend/env.vercel`** and are copied into the image as `/app/.env` by `Dockerfile.vercel`.

You do **not** need Vercel dashboard env vars or `.env.production` for the API to boot.

```
backend/env.vercel  →  Docker COPY  →  /app/.env  →  entrypoint force-exports DB/secrets
```

Domain: https://global-social-connections.vercel.app

> Warning: baking `DATABASE_URL` / secrets into a committed file puts them in the image and git history. Prefer rotating credentials if the repo is public.

If `/healthz` returns `FUNCTION_INVOCATION_FAILED`, the Go process crashed on boot (almost always **database connect**).

Use the Supabase **pooler** URL in `backend/env.vercel` (direct `db.*.supabase.co` often fails from cloud runtimes).

**Important:** Remove any conflicting `DATABASE_URL` from the Vercel project Environment Variables, or set it to the same pooler URL. Dashboard env overrides `godotenv`; the entrypoint now re-applies baked `/app/.env` for `DATABASE_URL` so a stale dashboard value cannot kill the container.

```
Browser  →  https://global-social-connections.vercel.app
              ├─ /api/* , /healthz  →  backend (Dockerfile.vercel)
              └─ /*                →  frontend (Dockerfile.vercel)
```

## 1. Env files

Production for Vercel is **`backend/env.vercel`** (committed, baked into Docker).  
Edit that file to change DB URL / secrets, then redeploy.

Optional local dry-run still uses `backend/.env.vercel` with `docker-compose.vercel.yml`.

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

Required dashboard env: **none for basic boot** (values come from `backend/env.vercel` inside the image).  
You may still set overrides in the Vercel UI if you want.

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
