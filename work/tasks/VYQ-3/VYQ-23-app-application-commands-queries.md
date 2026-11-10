---
id: VYQ-23
type: task
title: "[go] app.Application Commands and Queries"
status: "open"
priority: "major"
area: "go"
epic: VYQ-3
depends_on: [VYQ-19, VYQ-20, VYQ-21, VYQ-22]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-23 — [go] app.Application Commands and Queries

## Scope

`app.Application{Commands, Queries}` as in Wild Workouts. Handlers leave already wrapped in decorators. No SQL or HTTP wiring.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/app/` green
- [ ] `internal/links/app` does not import `database/sql` or `net/http`

## Evidence & Notes

_Pending._
