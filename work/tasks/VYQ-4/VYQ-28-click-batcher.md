---
id: VYQ-28
type: task
title: "[go] click_batcher"
status: "open"
priority: "major"
area: "go"
epic: VYQ-4
depends_on: [VYQ-22]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-28 — [go] click_batcher

## Scope

Bounded channel; flush every N events or T seconds; drain on shutdown. Full channel: drop the click and count it in a metric. Tests with `testing/synctest`. Analytics are approximate.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/adapters/ -run Batcher` green
- [ ] a shutdown with pending events persists them

## Evidence & Notes

_Pending._
