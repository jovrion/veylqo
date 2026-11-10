---
id: VYQ-23
type: task
title: "[go] app.Application Commands and Queries"
status: "done"
priority: "major"
area: "go"
epic: VYQ-3
depends_on: [VYQ-19, VYQ-20, VYQ-21, VYQ-22]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-23 — [go] app.Application Commands and Queries

## Scope

`app.Application{Commands, Queries}` as in Wild Workouts. Handlers leave already wrapped in decorators. No SQL or HTTP wiring.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/app/` green
- [x] `internal/links/app` does not import `database/sql` or `net/http`

## Evidence & Notes

- `gofmt -s -w . && go vet ./... && go test ./... && go test ./internal/archtest/...` — all packages ok, including `internal/links/app`.
- `go test ./internal/links/app/ -count=1` — ok. `TestApplicationWiresCommandsAndQueries` shortens, resolves, gets, counts, records one click, and deletes through `Application`. Resolve does not enqueue a click. A deleted code resolves as `gone`. Metrics observe `ShortenLink`, `ResolveLink`, `GetLink`, `LinkStats`, `RecordClick`, and `DeleteLink`, so each handler left already wrapped. The log has `handler=ShortenLink` and does not contain the destination or `token=secret`.
- `TestHandlersLeaveWrapped`: a panic in `Get` and a panic in `Enqueue` both return `decorator.ErrHandlerPanicked`. Metrics class is `panic`. The log records `error_class=panic` and not the URL, query, or IP from the panic value.
- `TestNewApplicationDoesNotPanicOnNilDependencies`: zero `Dependencies` still returns non-nil handlers. `Shorten` fails in `Handle` instead of panicking.
- `TestAppDoesNotImportHTTPOrSQL` parses every non-test file under `internal/links/app` and rejects `net/http` and `database/sql`. The app root also rejects `adapters`, `ports`, and `service`. SQL and HTTP stay out of this package. Adapter assembly remains `service.NewApplication` (VYQ-37).
