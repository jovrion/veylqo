---
id: VYQ-39
type: task
title: "[go] HTTP component tests"
status: "open"
priority: "major"
area: "go"
epic: VYQ-6
depends_on: [VYQ-38]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-39 — [go] HTTP component tests

## Scope

httptest (and Go 1.27 `NewTestServer` if it helps) against the component app: shorten → resolve 302 → stats. No Docker.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/service/ -run Component` green
- [ ] the shorten+resolve flow uses no real network

## Evidence & Notes

_Pending._
