---
id: VYQ-12
type: task
title: "[go] archtest for layers and stdlib-only"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-1
depends_on: [VYQ-11]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-12 — [go] archtest for layers and stdlib-only

## Scope

Package `internal/archtest` using stdlib `go/parser` and `go/build`. Fails if `domain` imports `app|adapters|ports|service` or a third-party module; if a package outside `cmd/` imports a driver; if `encoding/json` v1 or a third-party router appears.

On an almost empty tree the test must pass (no violations).

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/archtest/...` exits 0
- [ ] the test names the stdlib-only rule and the layer rule explicitly

## Evidence & Notes

_Pending._
