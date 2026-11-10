---
id: VYQ-36
type: task
title: "[go] Contract test spec ↔ handlers"
status: "open"
priority: "major"
area: "go"
epic: VYQ-5
depends_on: [VYQ-33, VYQ-34]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-36 — [go] Contract test spec ↔ handlers

## Scope

Read `veylqo.json` with `encoding/json/v2` and assert every spec path and response code has a handler, and that there are no extra undocumented routes.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test` of the contract is green
- [ ] removing a path from the spec or the mux fails the test

## Evidence & Notes

_Pending._
