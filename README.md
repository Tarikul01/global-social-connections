# FindNeighbour / GSC

Monorepo with two projects + Docker Compose (Postgres):

```
GSC/
├── backend/           # Go JSON API
├── frontend/          # React (Vite) SPA
├── docker-compose.yml # postgres + backend + frontend
└── README.md
```

## Quick start (Docker)

```bash
# Ensure backend/.env exists (copy from example if needed)
cp -n backend/.env.example backend/.env

docker compose up --build
```

| Service   | URL                    |
|-----------|------------------------|
| Frontend  | http://localhost:3000  |
| Backend   | http://localhost:8080  |
| Postgres  | localhost:5433         |

Postgres credentials: `findneighbour` / `findneighbour` / db `findneighbour`.

## Local development (without Docker apps)

```bash
# 1) Database only
docker compose up -d postgres

# 2) Backend
cd backend
# DATABASE_URL should use localhost:5433
go run ./cmd/server

# 3) Frontend
cd frontend
npm install
npm run dev   # http://localhost:5173 (proxies /api → :8080)
```

## Backend

- Go + Gin + PostGIS
- API only: `/api/v1/...`
- See `backend/.env.example`

## Frontend

- React + TypeScript + Vite + Tailwind + Leaflet
- Talks to backend via `/api` (Vite proxy in dev, nginx in Docker)
