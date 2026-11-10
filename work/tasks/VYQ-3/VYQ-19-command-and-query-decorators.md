---
id: VYQ-19
type: task
title: "[go] Command and query decorators"
status: "open"
priority: "major"
area: "go"
epic: VYQ-3
depends_on: [VYQ-18]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-19 — [go] Command and query decorators

## Scope

`internal/common/decorator`: logging (`log/slog`) and metrics, no business logic. Order recovery → logging → metrics. Stable `Name()`. Do not log destination URLs or tokens.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/common/decorator/` green
- [ ] a panic in the handler becomes an error and does not crash the process

## Evidence & Notes

_Pending._
