---
id: VYQ-40
type: task
title: "[ops] Dockerfile and docker-compose"
status: "done"
priority: "major"
area: "ops"
epic: VYQ-6
depends_on: [VYQ-38]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-40 — [ops] Dockerfile and docker-compose

## Scope

Multi-stage Dockerfile, `CGO_ENABLED=0`, distroless. Compose: postgres + veylqo. Document `VEYLQO_ALLOWED_HOSTS`, `VEYLQO_BASE_URL`, keys.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `test -f Dockerfile -a -f docker-compose.yml`
- [x] the Dockerfile sets `CGO_ENABLED=0`

## Evidence & Notes

2026-11-12:

```
test -f Dockerfile -a -f docker-compose.yml
files: ok
grep -n CGO_ENABLED Dockerfile
2:# CGO_ENABLED=0 keeps the binary static (ADR-0003: modernc.org/sqlite and pgx).
16:ENV CGO_ENABLED=0
docker compose -f docker-compose.yml config
docker build -t veylqo:local .
gofmt -s -w .
go vet ./...
go test ./...
go test ./internal/archtest/...
```

`docker compose up -d` started Postgres healthy and veylqo listening. `GET /healthz` and `GET /readyz` returned 200. `POST /api/v1/links` returned 201 with a 7-character code. `GET /{code}` was `302 Found` with `Cache-Control: private, max-age=0` and a Location without a query string. Logs were `migrations applied` and `listening addr=[::]:8080`, with no destination, key, or token. The stack was removed with `docker compose down -v`.

`VEYLQO_BASE_URL` is documented in `docker-compose.yml` as unread by the process. Keys are distinct local placeholders. No `.env` was committed.
