---
id: VYQ-45
type: task
title: "[js] @veylqo/element vanilla Web Component"
status: "done"
priority: "critical"
area: "js"
epic: VYQ-7
depends_on: [VYQ-44]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-45 — [js] @veylqo/element vanilla Web Component

## Scope

`<veylqo-share url endpoint key>`. Shadow DOM, CSS parts, events `veylqo:shortened` and `veylqo:shared`. Accessible. Zero runtime dependencies. No Lit.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `pnpm --filter @veylqo/element test` green
- [x] `package.json` of element has `"dependencies": {}` or omits the key

## Evidence & Notes

2026-11-12. Implemented in `/Users/jarias/code/veylqo-js` (not committed, not pushed). `packages/element/src/index.js` now defines and registers the vanilla custom element `veylqo-share`, exports its class and event constants, and remains framework-free. Node test compatibility was kept by guarding DOM globals.

```bash
cd /Users/jarias/code/veylqo-js && corepack pnpm --filter @veylqo/element test
# tests 1, pass 1, fail 0, exit 0

node -e 'const p=require("/Users/jarias/code/veylqo-js/packages/element/package.json"); if ("dependencies" in p && Object.keys(p.dependencies||{}).length>0) process.exit(1)'
# exit 0 (dependencies key omitted)
```
