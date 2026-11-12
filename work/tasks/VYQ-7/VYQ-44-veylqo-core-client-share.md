---
id: VYQ-44
type: task
title: "[js] @veylqo/core: client and share()"
status: "done"
priority: "critical"
area: "js"
epic: VYQ-7
depends_on: [VYQ-43]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-44 — [js] @veylqo/core: client and share()

## Scope

Client from the spec (openapi-typescript as a dev dependency). `shorten()` with sessionStorage cache. `share()`: Web Share API and copy / WhatsApp / X / email fallback.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `pnpm --filter @veylqo/core test` green
- [x] `package.json` of core lists no UI runtime dependencies

## Evidence & Notes

2026-11-12. Implemented in `/Users/jarias/code/veylqo-js` (not committed, not pushed). Spec snapshot: `packages/core/openapi/veylqo.json`, copied from `api/openapi/veylqo.json`. Types: `openapi-typescript` 7.13.0 as a devDependency (`pnpm --filter @veylqo/core generate` → `packages/core/src/openapi.d.ts`). Hand-written `createClient().shorten()` posts `POST /api/v1/links` with the publishable key and caches only `{ code }` in sessionStorage. `share()` uses `navigator.share`, otherwise copy / WhatsApp / X / email. No UI runtime dependency.

```
cd /Users/jarias/code/veylqo-js && corepack pnpm --filter @veylqo/core test
# tests 14, pass 14, fail 0, exit 0

cd /Users/jarias/code/veylqo-js && corepack pnpm -r test
# core pass 14, element pass 1, cdn pass 1, fail 0

node -e 'const p=require("/Users/jarias/code/veylqo-js/packages/core/package.json"); if (p.dependencies) process.exit(1); const ui=["react","vue","lit","svelte","preact","solid-js"]; for (const n of ui) if (n in (p.devDependencies||{})) process.exit(1)'
# exit 0. dependencies absent. devDependencies: openapi-typescript ^7.13.0 only.
```
