---
id: VYQ-27
type: task
title: "[go] Postgres repository"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-4
depends_on: [VYQ-24, VYQ-25]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-27 — [go] Postgres repository

## Scope

`database/sql` adapter + pgx stdlib. Unique index on `code`; partial unique on `destination_hash` for links without a custom slug. Tests with `VEYLQO_TEST_PG_DSN` or `t.Skip`. UUID as type `uuid`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/adapters/ -run Postgres` green or documented skip
- [ ] package `adapters` (non-`_test`) does not import `github.com/jackc/pgx`

## Evidence & Notes

_Pending._
