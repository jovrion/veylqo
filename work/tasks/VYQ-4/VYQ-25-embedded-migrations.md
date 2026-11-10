---
id: VYQ-25
type: task
title: "[go] Embedded migrations"
status: "open"
priority: "major"
area: "go"
epic: VYQ-4
depends_on: [VYQ-12]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-25 — [go] Embedded migrations

## Scope

SQL embedded with `embed.FS`, `schema_migrations` table, one transaction per migration, advisory lock on Postgres. Directories `migrations/{postgres,sqlite}/`. No goose.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test` of the migrator (tmp sqlite) applies and is idempotent
- [ ] no `pressly/goose` or similar dependency in `go.mod`

## Evidence & Notes

_Pending._
