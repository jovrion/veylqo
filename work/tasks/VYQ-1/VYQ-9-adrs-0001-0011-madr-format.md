---
id: VYQ-9
type: task
title: "[docs] ADRs 0001–0011 in MADR format"
status: "open"
priority: "critical"
area: "docs"
epic: VYQ-1
depends_on: [VYQ-8]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-9 — [docs] ADRs 0001–0011 in MADR format

## Scope

Write `docs/adr/0000-template.md` and ADRs 0001–0011, status Accepted, covering: architecture decision records; Clean Architecture and the Wild Workouts patterns that apply; stdlib policy (Go 1.27, `encoding/json/v2`, `uuid`) and the two driver exceptions; OpenAPI JSON contract without codegen; Postgres + SQLite via `database/sql`; internal UUIDv7 and public base62 code; 302 redirects; URL cleaning; async clicks and privacy; abuse prevention; separate repos and contract sync.

No product code. One ADR per file, kebab-case names. English.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `ls docs/adr/0000-template.md docs/adr/0001-*.md … docs/adr/0011-*.md` lists 12 files
- [ ] each ADR has Status: Accepted (the template may say Proposed/Accepted)
- [ ] `make work-validate` stays green

## Evidence & Notes

_Pending._
