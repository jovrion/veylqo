---
id: VYQ-49
type: task
title: "[js] GitHub Actions CI for veylqo-js"
status: "done"
priority: "major"
area: "js"
epic: VYQ-7
depends_on: [VYQ-47]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-49 — [js] GitHub Actions CI for veylqo-js

## Scope

Workflow `.github/workflows/ci.yml` in `jovrion/veylqo-js`: frozen lockfile,
`pnpm lint`, `pnpm build`, and `pnpm test`. The linter is
`scripts/lint.mjs` (`node --check`, no UI runtime dependencies in
`core` or `element`, and a clean `openapi-typescript` generate). No new
runtime dependency. ADR-0011 still holds: the contract stays in this repo.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `test -f /Users/jarias/code/veylqo-js/.github/workflows/ci.yml`
- [x] the workflow invokes `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm build`, and `pnpm test`
- [x] `cd /Users/jarias/code/veylqo-js && corepack pnpm lint && corepack pnpm build && corepack pnpm test`

## Evidence & Notes

2026-11-12. Implemented in `/Users/jarias/code/veylqo-js` (not committed, not pushed). Job name is `test`, which is the status-check context for VYQ-50. Node 22 in CI; pnpm 10.22.0 via `corepack install` from `packageManager`. No linter package added.

```
test -f /Users/jarias/code/veylqo-js/.github/workflows/ci.yml
# exit 0

grep -n -E 'pnpm (install --frozen-lockfile|lint|build|test)' /Users/jarias/code/veylqo-js/.github/workflows/ci.yml
# 36:        run: pnpm install --frozen-lockfile
# 39:        run: pnpm lint
# 42:        run: pnpm build
# 45:        run: pnpm test

cd /Users/jarias/code/veylqo-js && corepack pnpm lint && corepack pnpm build && corepack pnpm test
# lint: 13 files checked, openapi types match
# packages/cdn build: Done
# core pass 14, element pass 1, cdn pass 3, fail 0
# exit 0
```
