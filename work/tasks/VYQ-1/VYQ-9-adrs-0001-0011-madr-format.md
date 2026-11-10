---
id: VYQ-9
type: task
title: "[docs] ADRs 0001–0011 in MADR format"
status: "done"
priority: "critical"
area: "docs"
epic: VYQ-1
depends_on: [VYQ-8]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-9 — [docs] ADRs 0001–0011 in MADR format

## Scope

Write `docs/adr/0000-template.md` and ADRs 0001–0011, status Accepted, covering: architecture decision records; Clean Architecture and the Wild Workouts patterns that apply; stdlib policy (Go 1.27, `encoding/json/v2`, `uuid`) and the two driver exceptions; OpenAPI JSON contract without codegen; Postgres + SQLite via `database/sql`; internal UUIDv7 and public base62 code; 302 redirects; URL cleaning; async clicks and privacy; abuse prevention; separate repos and contract sync.

No product code. One ADR per file, kebab-case names. English.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `ls docs/adr/0000-template.md docs/adr/0001-*.md … docs/adr/0011-*.md` lists 12 files
- [x] each ADR has Status: Accepted (the template may say Proposed/Accepted)
- [x] `make work-validate` stays green

## Evidence & Notes

2026-11-10:

```
ls docs/adr/0000-template.md docs/adr/0001-*.md docs/adr/0002-*.md \
  docs/adr/0003-*.md docs/adr/0004-*.md docs/adr/0005-*.md \
  docs/adr/0006-*.md docs/adr/0007-*.md docs/adr/0008-*.md \
  docs/adr/0009-*.md docs/adr/0010-*.md docs/adr/0011-*.md
```

listed 12 files. `ls docs/adr/*.md | wc -l` printed 12.

`rg -n "^Status:" docs/adr` — `0001` through `0011` are `Status: Accepted`.
`0000-template.md` is `Status: Proposed | Accepted | Rejected | Deprecated | Superseded by [ADR-NNNN](...)`.

`make work-validate` → `work: OK (47 items, 0 warnings)`.
