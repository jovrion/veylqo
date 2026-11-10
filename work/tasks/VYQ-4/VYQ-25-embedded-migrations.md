---
id: VYQ-25
type: task
title: "[go] Embedded migrations"
status: "done"
priority: "major"
area: "go"
epic: VYQ-4
depends_on: [VYQ-12]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-25 — [go] Embedded migrations

## Scope

SQL embedded with `embed.FS`, `schema_migrations` table, one transaction per migration, advisory lock on Postgres. Directories `migrations/{postgres,sqlite}/`. No goose.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test` of the migrator (tmp sqlite) applies and is idempotent
- [x] no `pressly/goose` or similar dependency in `go.mod`

## Evidence & Notes

`internal/migrate` embeds `migrations/{postgres,sqlite}/`. `//go:embed` cannot use `..`, so the directories live under that package rather than the module root. `Apply` records versions in `schema_migrations`, one transaction per file. Postgres takes `pg_advisory_lock` on the same connection for the whole call. SQLite does not. Startup wiring stays in VYQ-38.

`gofmt -s -w . && go vet ./... && go test ./... && go test ./internal/archtest/...` passed. `go test ./internal/migrate/ -count=1` covers a temporary SQLite file (`TestApplySQLiteEmbedded`: apply, partial unique index, second apply leaves `applied_at` unchanged) and the Postgres lock on a stub driver.

`go.mod` requires `modernc.org/sqlite v1.62.0` (ADR-0003). The blank import is only in `internal/migrate/migrate_test.go`. No `pressly/goose`, `golang-migrate`, or `dbmate`. `github.com/google/uuid` is an indirect dependency of the SQLite driver and is not imported.
