---
id: VYQ-19
type: task
title: "[go] Command and query decorators"
status: "done"
priority: "major"
area: "go"
epic: VYQ-3
depends_on: [VYQ-18]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-19 — [go] Command and query decorators

## Scope

`internal/common/decorator`: logging (`log/slog`) and metrics, no business logic. Order recovery → logging → metrics. Stable `Name()`. Do not log destination URLs or tokens.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/common/decorator/` green
- [x] a panic in the handler becomes an error and does not crash the process

## Evidence & Notes

- `go test ./internal/common/decorator/` — ok (also `gofmt -s -w .`, `go vet ./...`, `go test ./...`, `go test ./internal/archtest/...`).
- Panic: `TestPanicBecomesErrorAndDoesNotCrash` and `TestCommandPanicBecomesError`. The handler panics with a URL, a token, and an IP. `Handle` returns `ErrHandlerPanicked`, the process stays up, and the log records `error_class=panic` without that payload.
- Order is recovery, then logging, then metrics. Recovery is innermost so the panic is an error before it is logged or counted. `spyMetrics.logReady` fails if metrics runs before that log line.
- Logs and metric labels carry `Name()` and the error class only. A destination, query string, IP, or token in the command, the result, the error text, or the panic value is not recorded. An unsafe `Name()` is logged as `redacted`.
- No business rules and no `internal/links` import. `TestPackageDoesNotImportLinksOrHTTP` rejects `net/http`, `database/sql`, and third-party modules. Wrapping inside `app.Application` is VYQ-23.
