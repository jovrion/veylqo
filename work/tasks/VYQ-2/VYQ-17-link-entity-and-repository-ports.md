---
id: VYQ-17
type: task
title: "[go] Link entity and repository ports"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-2
depends_on: [VYQ-14, VYQ-15]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-17 — [go] Link entity and repository ports

## Scope

`NewLink`, `IsExpired(now)`, `Delete()`, `UnmarshalLinkFromDatabase`. Private fields. Interfaces `Repository` (`Add`, `Get`, `GetByDestination`, `Update(ctx, code, updateFn)`) and `ClickRepository`. Typed domain errors, no `panic`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/domain/link/` green
- [x] `NewLink` rejects an invalid destination or code
- [x] package `domain` does not import `net/http` or `database/sql`

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/domain/link/ -count=1
ok  github.com/jovrion/veylqo/internal/links/domain/link
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/links/domain/link
```

`go test ./internal/links/domain/link/ -run 'TestNewLinkRejectsInvalidCodeOrDestination|TestDomainDoesNotImportHTTPOrSQL' -count=1 -v` passed. `NewLink` returns `ErrInvalidCode` for `api` and a short code, and `ErrInvalidDestination` for `javascript:alert(1)` (reason `scheme`) and `http://127.0.0.1/?token=secret` with blocking on (reason `private`). The error text does not include the URL, the query, or the address. `TestDomainDoesNotImportHTTPOrSQL` parses production files under `internal/links/domain` and fails on `net/http` or `database/sql`.

`Delete` takes the same clock as `IsExpired(now)` so a test does not call `time.Now`. A second delete returns `ErrInvalidLink` and keeps the first timestamp. `UnmarshalLinkFromDatabase` keeps a historical row that `NewLink` rejects (`api`, loopback, expiry already past) and does not rewrite the stored destination. Corruption (nil id, empty code, non-http(s) destination, zero `CreatedAt`) is still an error. No `panic`.

`Repository` is `Add`, `Get`, `GetByDestination`, `Update(ctx, code, updateFn)`. `ClickRepository` is `Add(ctx, code, n)` and `Count`. A click is a count for a public code; the method set has no IP, token, or destination. The in-memory adapter is VYQ-24. Allowlists and cleaning stay out of this package.
