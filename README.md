# GSC — Global Social Connections

**Product name:** FindNeighbour  
**Project / brand acronym:** **GSC = Global Social Connections**

Location-aware professional networking and meetup platform — discover nearby professionals, connect, chat, and meet up. Single-repo Go MVC app, deployable to Vercel (or any long-running Go host).

## About the name

| Term | Meaning |
|------|---------|
| **GSC** | **Global Social Connections** |
| **FindNeighbour** | App product name (UI, routes, database) |

Folder: `Projects/GSC` · App module: `github.com/findneighbour/findneighbour`

## Stack

- **Backend:** Go + Gin (MVC: Controller → Service → Repository)
- **Frontend:** Go HTML templates + HTMX + Alpine.js + Tailwind CSS
- **Database:** PostgreSQL + PostGIS (local Docker or Supabase)
- **Storage:** Supabase Storage
- **Realtime:** HTMX polling + PostgreSQL (no Redis required)
- **Maps:** OpenStreetMap (Leaflet) — no API key required
- **Deploy:** Vercel serverless Go entry (`api/index.go`) or `cmd/server` locally

## Dual surface: Web UI + Mobile API

One Gin engine (`internal/router.SetupRouter`) serves both:

| Surface | Base URL | Content-Type |
|---------|----------|--------------|
| Web MVC / HTMX | `https://globalconnection.vercel.app/` | HTML |
| Mobile REST | `https://globalconnection.vercel.app/api/v1/...` | JSON (+ CORS) |

Shared `UserService` backs both HTML (`/explore`) and JSON (`/api/v1/users/nearby`, `/api/v1/users/:id`) — same PostGIS queries.

```bash
# Example mobile call
curl "https://globalconnection.vercel.app/api/v1/users/nearby?lat=23.81&lng=90.41&radius_km=10"
```

## Quick Start (Local)

```bash
# 1. Start THIS project's PostgreSQL (port 5433 — separate from NestCareVest on 5432)
docker compose up -d

# 2. Configure environment
# Set nothing for maps — OpenStreetMap works without an API key
cp .env.example .env
# Edit APP_SECRET and DATABASE_URL if needed

# 3. (Optional) Seed demo people on the map
docker exec -i gsc-postgres-1 psql -U findneighbour -d findneighbour < migrations/002_demo_map_people.sql

# 4. Run the server
go run ./cmd/server

# 5. Open http://localhost:8080
```

**Database (this project):**

| Setting | Value |
|---------|--------|
| Container | `gsc-postgres-1` |
| Port | `5433` |
| Database | `findneighbour` |
| User / password | `findneighbour` / `findneighbour` |
| `DATABASE_URL` | `postgresql://findneighbour:findneighbour@localhost:5433/findneighbour?sslmode=disable` |

> NestCareVest Postgres on port **5432** is a different project. GSC uses its own Postgres on **5433**.

## Deploy to Vercel

1. Push this repo to GitHub  
2. Import project in [Vercel](https://vercel.com)  
3. Set environment variables from `.env.example`  
4. Use Supabase PostgreSQL connection string for `DATABASE_URL`  
5. Deploy — all routes go through `api/index.go`

```bash
npm i -g vercel
vercel --prod
```

## Project Structure

```
GSC/                          # Global Social Connections
├── api/index.go              # Vercel serverless entry
├── cmd/server/main.go        # Local FindNeighbour server
├── internal/                 # MVC modules
├── web/templates/            # HTML pages
├── web/static/               # CSS, JS
├── migrations/               # PostgreSQL + PostGIS schema
├── docs/ARCHITECTURE.md
├── docker-compose.yml
└── vercel.json
```

## Features

- Public Google Map homepage (browse people without login)
- Login only when adding yourself to the map or viewing a profile
- Online / offline markers on the map
- Profiles, follow, connection requests, chat
- Nearby discovery (PostGIS) + location privacy
- Events, join requests, event map / live location consent
- Notifications, block & report

## Run Tests

```bash
go test ./...
```

## Security Notes

- Sessions stored as HttpOnly cookies  
- CSRF on mutating requests  
- Location fuzzing for discovery (~200–500m)  
- Live location requires explicit consent  
- Rate limiting via PostgreSQL  

## License

MIT
