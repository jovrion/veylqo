---
id: VYQ-41
type: task
title: "[ops] CI e2e against Postgres"
status: "open"
priority: "normal"
area: "ops"
epic: VYQ-6
depends_on: [VYQ-40, VYQ-13]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-41 — [ops] CI e2e against Postgres

## Scope

CI job with a Postgres service and `VEYLQO_TEST_PG_DSN`. Postgres adapter tests stop skipping.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] the workflow has a job with Postgres
- [ ] the job exports `VEYLQO_TEST_PG_DSN`

## Evidence & Notes

_Pending._
