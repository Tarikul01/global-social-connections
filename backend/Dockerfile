# syntax=docker/dockerfile:1
# Local docker-compose backend (port 8080). Not used by Vercel.

FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server /app/server
RUN mkdir -p /app/data/uploads
ENV APP_ENV=production
ENV APP_PORT=8080
EXPOSE 8080
CMD ["/app/server"]
