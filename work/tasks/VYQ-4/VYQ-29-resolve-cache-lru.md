---
id: VYQ-29
type: task
title: "[go] resolve_cache LRU"
status: "open"
priority: "normal"
area: "go"
epic: VYQ-4
depends_on: [VYQ-21]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-29 — [go] resolve_cache LRU

## Scope

LRU decorator (`container/list`) with TTL over the ResolveLink read model. Invalidate on delete. Eviction and TTL tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test` of the cache is green
- [ ] after Delete, the next Resolve does not serve the stale value

## Evidence & Notes

_Pending._
