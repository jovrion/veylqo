---
id: VYQ-14
type: task
title: "[go] Code value object"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-2
depends_on: [VYQ-12]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-14 — [go] Code value object

## Scope

`internal/links/domain/link/code.go`: random 7-character base62 codes; custom slugs 3–64, regex `[a-zA-Z0-9_-]+`; reserved `api`, `healthz`, `readyz`, `metrics`, `debug`, `static`. Validating constructor. Table tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/domain/link/ -run Code` green
- [ ] slug `api` or `healthz` is an error
- [ ] `go test ./internal/archtest/...` stays green

## Evidence & Notes

_Pending._
