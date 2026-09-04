# FindNeighbour / Global Social Connections (GSC)

Location-aware professional networking: discover people on a map, see who’s online in realtime, and share live location — built as a Go API + React SPA.

**Repo:** [Tarikul01/global-social-connections](https://github.com/Tarikul01/global-social-connections)

```
GSC/
├── backend/                  # Go API (Dockerfile = local, Dockerfile.vercel = Vercel)
├── frontend/                 # React SPA (Dockerfile = local, Dockerfile.vercel = Vercel)
├── docker-compose.yml        # Local full stack + PostGIS
├── docker-compose.vercel.yml # Local test of Vercel Docker images
├── vercel.json               # Vercel multi-service deploy
├── DEPLOY.md
├── CONTRIBUTING.md
└── README.md
```

## Features

- Map discovery (Leaflet / OpenStreetMap) with profession filters
- Realtime nearby updates & online presence over **WebSocket** (`/api/v1/ws`)
- Live location sharing while logged in (socket events)
- Auth with cookie sessions
- Profile avatars: initials by default, photo when uploaded

## Local Docker

```bash
cp -n backend/.env.example backend/.env
# Set APP_SECRET (32+ chars)

docker compose up --build
```

| Service  | URL                    |
|----------|------------------------|
| Frontend | http://localhost:3000  |
| Backend  | http://localhost:8080  |
| Postgres | localhost:5433         |

## Vercel Docker deploy

Production URL: **https://global-social-connections.vercel.app**

All Vercel-specific files use a `.vercel` name (or `vercel.json`). See **[DEPLOY.md](./DEPLOY.md)**.

```bash
cp -n backend/.env.vercel.example backend/.env.vercel
# optional local dry-run:
docker compose -f docker-compose.vercel.yml up --build   # http://localhost:3080

# real deploy (external PostGIS + env in Vercel dashboard):
vercel --prod
```

## Local hot reload

```bash
docker compose up -d postgres
cd backend && go run ./cmd/server
cd frontend && npm install && npm run dev   # :5173
```

## Stack

| Layer | Tech |
|-------|------|
| Backend | Go, Gin, PostGIS, WebSocket |
| Frontend | React, TypeScript, Vite, Tailwind, Leaflet |
| Local | `docker-compose.yml` |
| Cloud | Vercel containers (`*.vercel*`) + external PostGIS |

## Contributing

See **[CONTRIBUTING.md](./CONTRIBUTING.md)**.
