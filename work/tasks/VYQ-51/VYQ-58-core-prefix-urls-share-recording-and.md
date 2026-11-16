---
id: VYQ-58
type: task
title: "[js] Core prefix URLs, share recording, and debug logs"
status: "open"
priority: "critical"
area: "js"
epic: VYQ-51
depends_on: [VYQ-56]
blocked_reason: null
updated: 2026-11-15
---

# VYQ-58 — [js] Core prefix URLs, share recording, and debug logs

## Scope

`jovrion/veylqo-js`, package `@veylqo/core`. Tracking stays in this
repo. Read ADR-0011 and ADR-0014. Copy `api/openapi/veylqo.json` into
`packages/core/openapi/veylqo.json` and regenerate types. Do not add a
UI runtime dependency. Do not add a route the spec does not list.

- `shorten()` posts `POST /api/v1/links` and returns `code`, `prefix`,
  and `short_url` from the response. The copied value is `short_url`.
  Do not assemble it from `window.location.origin`.
- Cache only `{ code, prefix, short_url }` in `sessionStorage`, and only
  for the no-slug, no-expiry case. On a cache hit, still call
  `POST /api/v1/links/{code}/shares` so a second copy is counted. A
  failed share report does not fail the copy.
- Optional `prefix` is sent only when the caller set one. Otherwise the
  server derives it.
- Add a logger with `prod` (default) and `debug`. `prod` is silent on
  success and, on failure, logs status and slug only. `debug` may log
  the problem title, slug, and status. Neither mode logs the publishable
  key or a destination query string.
- `copy()` writes the short URL. This path does not call `navigator.share`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `pnpm --filter @veylqo/core test` in `jovrion/veylqo-js` exits 0, including a cache-hit test that posts `/shares` and a log test that never prints the publishable key
- [ ] `packages/core/package.json` has no UI runtime dependency
- [ ] `pnpm lint` in `jovrion/veylqo-js` exits 0

## Evidence & Notes
