---
id: VYQ-43
type: task
title: "[js] pnpm scaffold for jovrion/veylqo-js"
status: "done"
priority: "major"
area: "js"
epic: VYQ-7
depends_on: [VYQ-31]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-43 — [js] pnpm scaffold for jovrion/veylqo-js

## Scope

Repo `jovrion/veylqo-js`: pnpm workspaces, MIT LICENSE, its own AGENTS.md, remote `git@github.com-unisimon:jovrion/veylqo-js.git`. Empty packages `core`, `element`, `cdn`. Changesets. Tracking is closed in this repo.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] the repo has `pnpm-workspace.yaml` and `packages/{core,element,cdn}`
- [x] `pnpm -r test` does not fail on the empty tree or a smoke test

## Evidence & Notes

2026-11-12. Scaffold in `/Users/jarias/code/veylqo-js` (not committed, not pushed). Origin `git@github.com-unisimon:jovrion/veylqo-js.git`. pnpm 10.22.0 via corepack (`packageManager` pin). Packages `@veylqo/core`, `@veylqo/element`, `@veylqo/cdn` are private empty scaffolds with a Node `node --test` smoke test. `@changesets/cli` 2.31.1 is a root devDependency. `.changeset/config.json` access is `public`.

```
test -f /Users/jarias/code/veylqo-js/pnpm-workspace.yaml \
  && test -d /Users/jarias/code/veylqo-js/packages/core \
  && test -d /Users/jarias/code/veylqo-js/packages/element \
  && test -d /Users/jarias/code/veylqo-js/packages/cdn
# exit 0

git -C /Users/jarias/code/veylqo-js remote get-url origin
# git@github.com-unisimon:jovrion/veylqo-js.git

cd /Users/jarias/code/veylqo-js && pnpm -r test
# packages/core, packages/element, packages/cdn: pass 1, fail 0
# exit 0
```
