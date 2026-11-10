---
id: VYQ-43
type: task
title: "[js] pnpm scaffold for jovrion/veylqo-js"
status: "open"
priority: "major"
area: "js"
epic: VYQ-7
depends_on: [VYQ-31]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-43 — [js] pnpm scaffold for jovrion/veylqo-js

## Scope

Repo `jovrion/veylqo-js`: pnpm workspaces, MIT LICENSE, its own AGENTS.md, remote `git@github.com-unisimon:jovrion/veylqo-js.git`. Empty packages `core`, `element`, `cdn`. Changesets. Tracking is closed in this repo.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] the repo has `pnpm-workspace.yaml` and `packages/{core,element,cdn}`
- [ ] `pnpm -r test` does not fail on the empty tree or a smoke test

## Evidence & Notes

_Pending._
