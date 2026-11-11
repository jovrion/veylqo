---
id: VYQ-27
type: task
title: "[go] Postgres repository"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-4
depends_on: [VYQ-24, VYQ-25]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-27 — [go] Postgres repository

## Scope

`database/sql` adapter + pgx stdlib. Unique index on `code`; partial unique on `destination_hash` for links without a custom slug. Tests with `VEYLQO_TEST_PG_DSN` or `t.Skip`. UUID as type `uuid`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/adapters/ -run Postgres` green or documented skip
- [x] package `adapters` (non-`_test`) does not import `github.com/jackc/pgx`

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/adapters/ -count=1 -race -run Postgres
ok github.com/jovrion/veylqo/internal/links/adapters
go test ./internal/archtest/... -count=1
ok github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok (archtest, decorator, errors, adapters, app, command, query, domain/link, migrate)
```

`VEYLQO_TEST_PG_DSN` was set for that run. An unset DSN skips with `t.Skip`; that path was not the run above.

`PostgresRepository` in `internal/links/adapters/link_postgres_repository.go` implements `link.Repository` with `database/sql`. The internal key is bound and scanned as `uuid.UUID`. `TestPostgresStoresIDAsUUID` checks `pg_typeof(id)` is `uuid`. Timestamps are Unix milliseconds; a zero expiry or deletion is NULL. `destination_hash` is SHA-256 of the stored destination string. The adapter does not clean. Add maps unique violations on `links_code` and `links_destination_hash_plain` to a conflict and does not wrap the driver error. Update uses `SELECT FOR UPDATE` across the load, `updateFn`, and the store. `TestPostgresRepository` calls `RunLinkRepositorySuite`; the cases are not copied.

The driver blank import is only in `link_postgres_repository_test.go`. `TestPostgresAdapterDoesNotImportDriver` parses every non-`_test.go` file in the package and rejects `github.com/jackc/pgx`. The only non-test mention is a comment. Production wiring of the blank import stays in `cmd/veylqo` (VYQ-38). The new `require` is `github.com/jackc/pgx/v5` (ADR-0003). No other module was added.
