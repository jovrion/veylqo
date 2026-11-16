---
id: VYQ-55
type: task
title: "[go] Attribution migrations and adapters"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-51
depends_on: [VYQ-54]
blocked_reason: null
updated: 2026-11-15
---

# VYQ-55 — [go] Attribution migrations and adapters

## Scope

Storage behind the domain interfaces (ADR-0005, ADR-0013). Adapters use
`database/sql` only. No new `require`. No driver import outside `cmd`
and adapter `*_test.go`. No goose.

- Embedded migration `0002` in both `migrations/sqlite/` and
  `migrations/postgres/`. Add the site and the event storage the domain
  interfaces require. Store a hash of the publishable key, not a form
  that logs can reprint. Index events so a stats read does not scan a
  visit log. Dialect differences stay in the migration directory and
  the adapter.
- A link row records its site and its prefix. Idempotent reuse is scoped
  to the site. Two sites shortening the same destination do not share a code.
- Memory, SQLite, and Postgres implement the same suite: create a site,
  reject a foreign site's code, insert share and click events, and
  return counts plus `by_referrer` and `by_device`. A crash before flush
  may lose events. Do not add an outbox or a broker.
- The batcher accepts both event types, stays bounded, drops on overflow,
  and drains on shutdown. It must not block the redirect path.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/migrate/... ./internal/links/adapters/...` exits 0
- [ ] `go test ./internal/archtest/...` exits 0 and `go.mod` has no new `require`
- [ ] the shared suite covers a share and a click for one site and a miss for another site's code

## Evidence & Notes
