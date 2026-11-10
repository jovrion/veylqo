---
id: VYQ-46
type: task
title: "[js] CDN bundle"
status: "open"
priority: "major"
area: "js"
epic: VYQ-7
depends_on: [VYQ-45]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-46 — [js] CDN bundle

## Scope

IIFE/ESM for `<script src>` (jsDelivr/unpkg). A static HTML file with no bundler registers the custom element.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] the CDN artifact exists after `pnpm --filter @veylqo/cdn build`
- [ ] an example HTML loads the script and defines `veylqo-share`

## Evidence & Notes

_Pending._
