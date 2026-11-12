---
id: VYQ-39
type: task
title: "[go] HTTP component tests"
status: "done"
priority: "major"
area: "go"
epic: VYQ-6
depends_on: [VYQ-38]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-39 — [go] HTTP component tests

## Scope

httptest (and Go 1.27 `NewTestServer` if it helps) against the component app: shorten → resolve 302 → stats. No Docker.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/service/ -run Component` green
- [x] the shorten+resolve flow uses no real network

## Evidence & Notes

2026-11-12:

```
gofmt -s -w .
go vet ./...
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/links/service
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
go test ./internal/links/service/ -run Component -count=1
ok  github.com/jovrion/veylqo/internal/links/service
```

`TestComponentShortenResolveStats` builds `NewComponentTestApplication` (memory, `DB == nil`) and drives `POST /api/v1/links` → `GET /{code}` → `GET /api/v1/links/{code}/stats` through `httptest.NewTestServer` without `Start`. The test fails if `Server.Listener` is set. The client uses `http.ErrUseLastResponse`, so the 302 is not followed and the destination path is not requested. Resolve is 302, not 301, with `Cache-Control: private, max-age=0` and the stored destination after tracking-parameter stripping. Stats waits for the batcher flush and then reports one click. No Docker.
