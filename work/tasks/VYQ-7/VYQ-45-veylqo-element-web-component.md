---
id: VYQ-45
type: task
title: "[js] @veylqo/element vanilla Web Component"
status: "open"
priority: "critical"
area: "js"
epic: VYQ-7
depends_on: [VYQ-44]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-45 — [js] @veylqo/element vanilla Web Component

## Scope

`<veylqo-share url endpoint key>`. Shadow DOM, CSS parts, events `veylqo:shortened` and `veylqo:shared`. Accessible. Zero runtime dependencies. No Lit.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `pnpm --filter @veylqo/element test` green
- [ ] `package.json` of element has `"dependencies": {}` or omits the key

## Evidence & Notes

_Pending._
