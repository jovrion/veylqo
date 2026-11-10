---
id: VYQ-26
type: task
title: "[go] SQLite repository"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-4
depends_on: [VYQ-24, VYQ-25]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-26 — [go] SQLite repository

## Scope

`database/sql` adapter + `modernc.org/sqlite`. The driver `import _` belongs in `cmd/veylqo`; the adapter test may register it only in `*_test.go` or a test helper. Timestamps as INTEGER ms. Passes the shared suite.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/adapters/ -run SQLite` green
- [ ] package `adapters` (non-`_test`) does not import `modernc.org/sqlite`

## Evidence & Notes

_Pending._
