---
id: VYQ-26
type: task
title: "[go] SQLite repository"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-4
depends_on: [VYQ-24, VYQ-25]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-26 — [go] SQLite repository

## Scope

`database/sql` adapter + `modernc.org/sqlite`. The driver `import _` belongs in `cmd/veylqo`; the adapter test may register it only in `*_test.go` or a test helper. Timestamps as INTEGER ms. Passes the shared suite.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/adapters/ -run SQLite` green
- [x] package `adapters` (non-`_test`) does not import `modernc.org/sqlite`

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/adapters/ -count=1 -run SQLite
ok github.com/jovrion/veylqo/internal/links/adapters
go test ./internal/links/adapters/ -count=1 -race -run SQLite
ok github.com/jovrion/veylqo/internal/links/adapters
go test ./internal/archtest/... -count=1
ok github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok (archtest, decorator, errors, adapters, app, command, query, domain/link, migrate)
```

`SQLiteRepository` in `internal/links/adapters/link_sqlite_repository.go` implements `link.Repository` with `database/sql`. Timestamps are Unix milliseconds; a zero expiry or deletion is NULL. The internal UUID is canonical text. `destination_hash` is SHA-256 of the stored destination string. The adapter does not clean. Update holds one lock across load, `updateFn`, and store. `TestSQLiteRepository` calls `RunLinkRepositorySuite`; the cases are not copied.

The driver blank import is only in `link_sqlite_repository_test.go`. `TestSQLiteAdapterDoesNotImportDriver` parses every non-`_test.go` file in the package and rejects `modernc.org/sqlite`. The only non-test mention is a comment. Production wiring of the blank import stays in `cmd/veylqo` (VYQ-38). No new `require`.
