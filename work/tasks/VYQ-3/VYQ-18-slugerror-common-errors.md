---
id: VYQ-18
type: task
title: "[go] SlugError in internal/common/errors"
status: "open"
priority: "major"
area: "go"
epic: VYQ-3
depends_on: [VYQ-12]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-18 — [go] SlugError in internal/common/errors

## Scope

Errors with a slug: `incorrect-input`, `not-found`, `gone`, `conflict`, `unauthorized`, `rate-limited`. Translated to HTTP in `ports`, not here. Wrapping tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/common/errors/` green
- [ ] each documented slug has a constructor

## Evidence & Notes

_Pending._
