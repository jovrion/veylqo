---
id: VYQ-34
type: task
title: "[go] GET /{code} 302 redirect"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-5
depends_on: [VYQ-21, VYQ-32]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-34 — [go] GET /{code} 302 redirect

## Scope

Outside `/api`. 302, `Cache-Control: private, max-age=0`, fires `RecordClick` without blocking. Reserved codes do not land here (`api`, `healthz`, `readyz`, `metrics`). Never 301.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] an httptest receives 302 and Location of the destination
- [ ] an expired code is 410, a missing one 404
- [ ] no test expects 301

## Evidence & Notes

_Pending._
