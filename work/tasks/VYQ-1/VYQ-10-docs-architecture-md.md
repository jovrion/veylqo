---
id: VYQ-10
type: task
title: "[docs] docs/architecture.md"
status: "done"
priority: "major"
area: "docs"
epic: VYQ-1
depends_on: [VYQ-9]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-10 — [docs] docs/architecture.md

## Scope

Architecture document: `domain/app/adapters/ports/service` layer diagram, shorten flow and resolve flow (302 + non-blocking RecordClick), security limits, and what is out of the MVP. Points at the ADRs; does not duplicate them. English.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `test -f docs/architecture.md`
- [x] the document names the five layers and the 302 redirect
- [x] `make work-validate` stays green

## Evidence & Notes

2026-11-10:

- `test -f docs/architecture.md` → `FILE_OK`
- `grep -E '^\| \`(domain|app|adapters|ports|service)\`' docs/architecture.md` lists all five layers. `grep -n '302' docs/architecture.md` names the 302 redirect (including `never 301`).
- `make work-validate` → `work: OK (47 items, 0 warnings)`
