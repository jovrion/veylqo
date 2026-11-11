---
id: VYQ-37
type: task
title: "[go] service.NewApplication"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-6
depends_on: [VYQ-23, VYQ-26, VYQ-27, VYQ-28, VYQ-30]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-37 — [go] service.NewApplication

## Scope

Choose the adapter from `VEYLQO_DB_URL` (`postgres://` or `sqlite://`). `NewComponentTestApplication` uses memory. Only place, together with `cmd/`, where pieces are assembled.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/service/` green
- [x] a sqlite DSN wires the sqlite adapter without touching Postgres

## Evidence & Notes

2026-11-11:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/service/ -count=1
ok  github.com/jovrion/veylqo/internal/links/service
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/links/adapters
ok  github.com/jovrion/veylqo/internal/links/service
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
```

`TestSQLiteDSNWiresSQLiteAdapter` opens driver `sqlite` only, type-asserts `*adapters.SQLiteRepository` and `*adapters.SQLiteClicks`, and counts on the migrated sqlite file. The postgres driver is not imported in that test. `TestPostgresDSNDoesNotOpenSQLite` records driver `pgx` and does not open sqlite. Non-test `service` files do not import either driver (`TestServiceDoesNotImportDrivers`). `NewComponentTestApplication` uses memory and does not call `sql.Open`.
