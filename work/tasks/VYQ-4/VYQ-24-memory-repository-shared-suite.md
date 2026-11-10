---
id: VYQ-24
type: task
title: "[go] In-memory repository and shared suite"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-4
depends_on: [VYQ-17]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-24 — [go] In-memory repository and shared suite

## Scope

`link_memory_repository.go` plus a test suite every adapter must pass: Add, Get, GetByDestination, atomic Update, code collision, delete.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/adapters/ -run Memory` green
- [x] the suite lives in a file reused by SQLite and Postgres

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/adapters/ -count=1 -run Memory -v
PASS ok github.com/jovrion/veylqo/internal/links/adapters
go test ./internal/links/adapters/ -count=1 -race -run Memory
ok github.com/jovrion/veylqo/internal/links/adapters
go test ./internal/archtest/... -count=1
ok github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok (archtest, decorator, errors, adapters, app, command, query, domain/link)
```

`MemoryRepository` in `internal/links/adapters/link_memory_repository.go` implements `link.Repository`. It does not import a driver or `database/sql`. Add conflicts on public code (`ConflictError` reason `code`) and on a second non-custom destination (`reason` `destination`), including after delete. Get and GetByDestination return expired and deleted rows and compare destination strings exactly. Update holds one lock across load, `updateFn`, and store; a failed `updateFn` is not stored; a missing code does not call `updateFn`.

The shared suite is `RunLinkRepositorySuite` in `internal/links/adapters/link_repository_suite_test.go`. `TestMemoryRepository` only supplies `NewMemoryRepository`. SQLite (VYQ-26) and Postgres (VYQ-27) call the same function with an empty repository. It is not copied into the memory test.
