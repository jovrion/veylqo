---
id: VYQ-33
type: task
title: "[go] HTTP API ports"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-5
depends_on: [VYQ-23, VYQ-31, VYQ-32]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-33 — [go] HTTP API ports

## Scope

`internal/links/ports/http.go` implements the spec by hand on `ServeMux`. Maps slugs to 400/401/404/409/410/429. No business logic: only calls Commands/Queries.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/ports/` covers the spec HTTP codes
- [ ] `ports` does not import `database/sql`

## Evidence & Notes

_Pending._
