---
id: VYQ-17
type: task
title: "[go] Link entity and repository ports"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-2
depends_on: [VYQ-14, VYQ-15]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-17 — [go] Link entity and repository ports

## Scope

`NewLink`, `IsExpired(now)`, `Delete()`, `UnmarshalLinkFromDatabase`. Private fields. Interfaces `Repository` (`Add`, `Get`, `GetByDestination`, `Update(ctx, code, updateFn)`) and `ClickRepository`. Typed domain errors, no `panic`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/domain/link/` green
- [ ] `NewLink` rejects an invalid destination or code
- [ ] package `domain` does not import `net/http` or `database/sql`

## Evidence & Notes

_Pending._
