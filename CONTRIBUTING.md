# Contributing to FindNeighbour (GSC)

Thanks for helping improve **FindNeighbour / Global Social Connections** — a location-aware professional networking app (map discovery, realtime presence, live location).

## Project layout

```
GSC/
├── backend/            # Go + Gin + PostGIS JSON API (`/api/v1/*`, WebSocket `/api/v1/ws`)
├── frontend/           # React + TypeScript + Vite + Tailwind + Leaflet SPA
├── docker-compose.yml  # Postgres (PostGIS), backend, frontend
├── CONTRIBUTING.md
└── README.md
```

## Prerequisites

- **Go** 1.22+ (see `backend/go.mod`)
- **Node.js** 20+ and npm
- **Docker** + Docker Compose (for Postgres / full stack)
- Git

## Local setup

```bash
# 1) Clone
git clone git@github.com:Tarikul01/global-social-connections.git
cd global-social-connections

# 2) Env
cp -n backend/.env.example backend/.env
# Edit APP_SECRET (32+ chars) and DATABASE_URL if needed

# 3) Database
docker compose up -d postgres

# 4) Backend (from repo root or backend/)
cd backend
go run ./cmd/server
# → http://localhost:8080  health: /healthz

# 5) Frontend (new terminal)
cd frontend
npm install
npm run dev
# → http://localhost:5173  (proxies /api and WebSocket to :8080)
```

Full stack with Docker:

```bash
cp -n backend/.env.example backend/.env
docker compose up --build
# Frontend http://localhost:3000 · API http://localhost:8080
```

## How we develop

| Area | Notes |
|------|--------|
| API | JSON under `/api/v1/...` only — no new HTML templates |
| Realtime map | WebSocket `GET /api/v1/ws` — prefer one long-lived connection; push `nearby` snapshots |
| Auth | Cookie sessions + CSRF on mutating requests (`credentials: 'include'`) |
| Location | PostGIS; live sharing via `/api/v1/live-location*` then map refresh over WS |
| Avatars | Initials by default; uploaded photos served from `/api/v1/media/...` (local `data/uploads`) |
| Frontend | TypeScript + Tailwind; map UI in `frontend/src/pages/MapPage.tsx` |

### Useful paths

- Backend entry: `backend/cmd/server`
- Router: `backend/internal/router`
- Realtime hub: `backend/internal/realtime`
- Map socket hook: `frontend/src/hooks/useMapSocket.ts`
- Env template: `backend/.env.example`

## Branch & pull requests

1. Create a branch from `main` (or the default branch):  
   `git checkout -b feat/short-description`
2. Keep changes focused — one concern per PR when possible.
3. Test locally (backend builds, frontend `npm run build` or `tsc`, map + login smoke test).
4. Open a PR with:
   - **What** changed and **why**
   - How you tested
   - Screenshots for UI changes

### Commit style

Prefer short, imperative messages:

- `fix: use WebSocket presence for online count`
- `feat: local profile photo upload and media route`
- `docs: add CONTRIBUTING and refresh README`

Do not commit secrets (`.env`, keys, credentials).

## Code guidelines

- Match existing style in the files you touch; avoid drive-by refactors.
- Backend: clear package boundaries (`auth`, `discovery`, `realtime`, `location`, …).
- Frontend: reuse shared helpers (`UserAvatar`, `api` client) instead of duplicating marker/HTML.
- Do not reintroduce HTML/HTMX map pages — the React SPA is the UI.
- Prefer fixing root causes (e.g. reconnect storms) over logging noise.

## Reporting issues

Include:

- OS / Go / Node versions
- Steps to reproduce
- Relevant API or browser console errors
- Whether you used Docker or local `go run` / `npm run dev`

## License / conduct

Be respectful. By contributing you agree your work may be included in this repository under the same terms as the project.

Questions? Open an issue on [global-social-connections](https://github.com/Tarikul01/global-social-connections).
