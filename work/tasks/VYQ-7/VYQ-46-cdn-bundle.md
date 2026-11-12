---
id: VYQ-46
type: task
title: "[js] CDN bundle"
status: "done"
priority: "major"
area: "js"
epic: VYQ-7
depends_on: [VYQ-45]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-46 — [js] CDN bundle

## Scope

IIFE/ESM for `<script src>` (jsDelivr/unpkg). A static HTML file with no bundler registers the custom element.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] the CDN artifact exists after `pnpm --filter @veylqo/cdn build`
- [x] an example HTML loads the script and defines `veylqo-share`

## Evidence & Notes

- Implemented `@veylqo/cdn` build outputs in `packages/cdn/dist/`:
  - `veylqo-cdn.esm.js`
  - `veylqo-cdn.iife.js`
- Added static no-bundler example at `packages/cdn/examples/index.html` that:
  - loads the script via `<script src="../dist/veylqo-cdn.iife.js"></script>`
  - uses `<veylqo-share>`
  - checks `customElements.get("veylqo-share")`
- Verification commands (run in `jovrion/veylqo-js`):
  - `pnpm --filter @veylqo/cdn build`
  - `pnpm test`
  - `pnpm -r build`
