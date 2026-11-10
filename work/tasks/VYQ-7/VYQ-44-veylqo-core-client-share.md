---
id: VYQ-44
type: task
title: "[js] @veylqo/core: client and share()"
status: "open"
priority: "critical"
area: "js"
epic: VYQ-7
depends_on: [VYQ-43]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-44 — [js] @veylqo/core: client and share()

## Scope

Client from the spec (openapi-typescript as a dev dependency). `shorten()` with sessionStorage cache. `share()`: Web Share API and copy / WhatsApp / X / email fallback.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `pnpm --filter @veylqo/core test` green
- [ ] `package.json` of core lists no UI runtime dependencies

## Evidence & Notes

_Pending._
