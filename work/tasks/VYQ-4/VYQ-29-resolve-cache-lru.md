---
id: VYQ-29
type: task
title: "[go] resolve_cache LRU"
status: "done"
priority: "normal"
area: "go"
epic: VYQ-4
depends_on: [VYQ-21]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-29 — [go] resolve_cache LRU

## Scope

LRU decorator (`container/list`) with TTL over the ResolveLink read model. Invalidate on delete. Eviction and TTL tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test` of the cache is green
- [x] after Delete, the next Resolve does not serve the stale value

## Evidence & Notes

2026-11-11:

`adapters.ResolveCache` is an LRU (`container/list`) with TTL over successful `Get` results, keyed by public code. It implements `link.Repository`, so Resolve and Delete share it when composition wraps the store. A hit returns the stored link and does not choose expiry or a redirect. Misses are not cached. Any successful `Update` drops that code, which is how delete invalidates. `GetByDestination` and `Add` do not fill the cache. No destination, IP, or token is logged. Not wired into `service` or `cmd` (VYQ-37 does not depend on this task).

```
gofmt -s -w .
go vet ./...
go test ./...
go test ./internal/archtest/...
go test ./internal/links/adapters/ -count=1 -race -run ResolveCache -timeout 120s -v
```

`go test ./...` and `go test ./internal/archtest/...` passed. `TestResolveCacheEvictsLeastRecentlyUsed` and `TestResolveCacheTTLExpires` passed. `TestResolveCacheDeleteDropsStaleResolve` resolved `Ab3xyZ9`, deleted it through `command.DeleteLinkHandler` on the same cache, and the next `query.ResolveLinkHandler` returned slug `gone` with an empty destination. The inner `Get` count went from 1 to 2, so the stale destination was not served. Error text did not contain the stored query.
