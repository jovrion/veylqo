---
id: VYQ-37
type: task
title: "[go] service.NewApplication"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-6
depends_on: [VYQ-23, VYQ-26, VYQ-27, VYQ-28, VYQ-30]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-37 — [go] service.NewApplication

## Scope

Choose the adapter from `VEYLQO_DB_URL` (`postgres://` or `sqlite://`). `NewComponentTestApplication` uses memory. Only place, together with `cmd/`, where pieces are assembled.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/service/` green
- [ ] a sqlite DSN wires the sqlite adapter without touching Postgres

## Evidence & Notes

_Pending._
