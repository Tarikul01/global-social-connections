# FindNeighbour / Global Social Connections (GSC)

Location-aware professional networking: discover people on a map, see who’s online in realtime, and share live location — built as a Go API + React SPA.

**Repo:** [Tarikul01/global-social-connections](https://github.com/Tarikul01/global-social-connections)

```
GSC/
├── backend/            # Go JSON API + WebSocket presence
├── frontend/           # React (Vite) + Leaflet map
├── docker-compose.yml  # PostGIS + backend + frontend
├── CONTRIBUTING.md
└── README.md
```

## Features

- Map discovery (Leaflet / OpenStreetMap) with profession filters
- Realtime nearby updates & online presence over **WebSocket** (`/api/v1/ws`)
- Live location sharing while logged in
- Auth (register / login / logout) with cookie sessions
- Profile avatars: name initials by default, uploaded photo when available

## Quick start (Docker)

```bash
cp -n backend/.env.example backend/.env
# Set APP_SECRET to a random string (32+ characters)

docker compose up --build
```

| Service  | URL                    |
|----------|------------------------|
| Frontend | http://localhost:3000  |
| Backend  | http://localhost:8080  |
| Postgres | localhost:5433         |

DB user / password / database: `findneighbour` / `findneighbour` / `findneighbour`.

## Local development

```bash
# 1) Database only
docker compose up -d postgres

# 2) Backend
cd backend
# DATABASE_URL → localhost:5433 (see .env.example)
go run ./cmd/server

# 3) Frontend
cd frontend
npm install
npm run dev   # http://localhost:5173 (proxies /api + WS → :8080)
```

## Stack

| Layer    | Tech |
|----------|------|
| Backend  | Go, Gin, PostGIS, WebSocket |
| Frontend | React, TypeScript, Vite, Tailwind, Leaflet |
| Infra    | Docker Compose |

API is JSON-only under `/api/v1/...`. See `backend/.env.example` for configuration.

## Contributing

Please read **[CONTRIBUTING.md](./CONTRIBUTING.md)** for setup, branch/PR expectations, and project conventions.
