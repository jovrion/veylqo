---
id: VYQ-15
type: task
title: "[go] Destination value object"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-2
depends_on: [VYQ-12]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-15 — [go] Destination value object

## Scope

Destination URL: `http`/`https` only; reject `javascript:`, `data:`, and, when configured, private/loopback hosts via `net/netip`. Normalize (lowercase host, no fragment). Table tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/domain/link/ -run Destination` green
- [ ] `javascript:alert(1)` and `http://127.0.0.1/` (with blocking on) fail

## Evidence & Notes

_Pending._
