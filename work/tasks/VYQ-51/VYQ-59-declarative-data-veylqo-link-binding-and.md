---
id: VYQ-59
type: task
title: "[js] Declarative data-veylqo-link binding and examples"
status: "open"
priority: "critical"
area: "js"
epic: VYQ-51
depends_on: [VYQ-58]
blocked_reason: null
updated: 2026-11-15
---

# VYQ-59 — [js] Declarative data-veylqo-link binding and examples

## Scope

`@veylqo/element` and `@veylqo/cdn`, plus the existing examples. Read
ADR-0014. No React, Vue, Lit, or other UI runtime. Do not remove
`<veylqo-share>`. The admin token must not appear in an example.

- `initVeylqo()` reads `<meta name="veylqo:endpoint">`,
  `<meta name="veylqo:key">`, optional `<meta name="veylqo:prefix">`,
  and `<meta name="veylqo:mode">` (`prod` or `debug`).
- One document-level click listener handles `[data-veylqo-link="true"]`,
  including nodes added later. On an `<a>`, call `preventDefault`.
  Resolve the destination from `data-veylqo-url`, then a http(s) `href`,
  then `<link rel="canonical">`, then `<meta property="og:url">`, then
  `location.href`.
- Per element, `data-veylqo-prefix`, `data-veylqo-slug`, and
  `data-veylqo-clean` override the meta values. The action copies
  `short_url` and sets `data-veylqo-state` to `busy`, then `copied` or
  `error`. Dispatch `veylqo:shortened`, `veylqo:copied`, and
  `veylqo:error`. Do not navigate and do not open the share sheet.
- Missing endpoint or key fails closed. `debug` explains which meta is
  missing. `prod` does not print the key or the destination query.
  Initialization is idempotent.
- The CDN bundle calls `initVeylqo()` when `document` exists.
- Examples in `examples/html`, `examples/react`, `examples/vue`,
  `examples/svelte`, and `packages/cdn/examples` show the meta tags and
  `<a data-veylqo-link="true">`. They may keep `<veylqo-share>` as the
  older option. React notes stay on custom events for React before 19.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `pnpm test` in `jovrion/veylqo-js` exits 0, including a test that a click on `<a data-veylqo-link="true">` copies `short_url` and does not call `navigator.share`
- [ ] `pnpm -r build && pnpm lint` in `jovrion/veylqo-js` exits 0
- [ ] `examples/html` and `packages/cdn/examples` contain `data-veylqo-link` and `<meta name="veylqo:endpoint"`

## Evidence & Notes
