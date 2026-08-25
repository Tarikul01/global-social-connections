# Backend — FindNeighbour API

Go + Gin JSON API (no HTML views).

```bash
# From repo root: start Postgres
docker compose up -d postgres

# From this folder
cp -n .env.example .env
# Set DATABASE_URL to localhost:5433 for host runs
go run ./cmd/server
```

API: http://localhost:8080/api/v1/health
