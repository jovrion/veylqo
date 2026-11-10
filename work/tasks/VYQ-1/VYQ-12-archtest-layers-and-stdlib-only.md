---
id: VYQ-12
type: task
title: "[go] archtest for layers and stdlib-only"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-1
depends_on: [VYQ-11]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-12 — [go] archtest for layers and stdlib-only

## Scope

Package `internal/archtest` using stdlib `go/parser` and `go/build`. Fails if `domain` imports `app|adapters|ports|service` or a third-party module; if a package outside `cmd/` imports a driver; if `encoding/json` v1 or a third-party router appears.

On an almost empty tree the test must pass (no violations).

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/archtest/...` exits 0
- [x] the test names the stdlib-only rule and the layer rule explicitly

## Evidence & Notes

2026-11-10. Package `internal/archtest` reads files with `go/build` and imports with `go/parser`. The layer rule rejects `domain` imports of `app`, `adapters`, `ports`, `service`, or a third-party module (ADR-0002). The stdlib-only rule rejects `encoding/json` v1, a third-party router, and a driver import outside `cmd/veylqo` production files (ADR-0003). `*_test.go` and `//go:build ignore` files are not driver violations. The current tree has no violations.

```
go test -count=1 -v ./internal/archtest/...
# === RUN   TestLayerRule
# === RUN   TestLayerRule/layer_rule
# === RUN   TestStdlibOnlyRule
# === RUN   TestStdlibOnlyRule/stdlib-only_rule
# PASS
# ok  	github.com/jovrion/veylqo/internal/archtest
# exit 0

gofmt -s -w .
go vet ./...
go test ./...
# ok  	github.com/jovrion/veylqo/internal/archtest
# exit 0
```
