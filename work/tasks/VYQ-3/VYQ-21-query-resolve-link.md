---
id: VYQ-21
type: task
title: "[go] query ResolveLink"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-3
depends_on: [VYQ-17, VYQ-18]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-21 — [go] query ResolveLink

## Scope

Return the destination or `not-found` / `gone`. Does not record the click (that is `RecordClick`). Does not import HTTP.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/app/query/ -run Resolve` green
- [ ] an expired link is `gone`, a missing one `not-found`

## Evidence & Notes

_Pending._
