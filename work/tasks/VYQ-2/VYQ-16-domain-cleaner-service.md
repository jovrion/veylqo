---
id: VYQ-16
type: task
title: "[go] Domain Cleaner service"
status: "open"
priority: "major"
area: "go"
epic: VYQ-2
depends_on: [VYQ-15]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-16 — [go] Domain Cleaner service

## Scope

Strip `utm_*`, `fbclid`, `gclid`, `ref`, `tag`, and similar. Optional per-host rules (e.g. keep `/dp/ASIN`). `clean: false` option. Table tests. ADR-0008 once it exists.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/domain/link/ -run Clean` green
- [ ] a URL with `utm_source` comes out without that query

## Evidence & Notes

_Pending._
