.PHONY: dev db run test build migrate

dev: db run

db:
	docker compose up -d

run:
	go run ./cmd/server

build:
	go build -o bin/findneighbour ./cmd/server

test:
	go test ./...

migrate:
	docker compose exec postgres psql -U findneighbour -d findneighbour -f /docker-entrypoint-initdb.d/001_schema.sql
