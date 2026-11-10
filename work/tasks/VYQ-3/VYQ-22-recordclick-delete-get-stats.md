---
id: VYQ-22
type: task
title: "[go] RecordClick, DeleteLink, GetLink, LinkStats"
status: "open"
priority: "major"
area: "go"
epic: VYQ-3
depends_on: [VYQ-17, VYQ-18]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-22 — [go] RecordClick, DeleteLink, GetLink, LinkStats

## Scope

Command `RecordClick` (does not block the caller: accepts an async port). `DeleteLink`. Queries `GetLink` and `LinkStats` (counts, no PII).

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/app/...` green for these handlers
- [ ] `RecordClick` does not wait for the click repository to finish

## Evidence & Notes

_Pending._
