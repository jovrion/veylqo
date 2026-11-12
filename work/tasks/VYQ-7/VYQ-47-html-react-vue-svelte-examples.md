---
id: VYQ-47
type: task
title: "[js] html, react, vue, svelte examples"
status: "done"
priority: "minor"
area: "js"
epic: VYQ-7
depends_on: [VYQ-45]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-47 — [js] html, react, vue, svelte examples

## Scope

`examples/` with plain HTML, React, Vue, and Svelte using the custom element. Document React < 19. Do not publish framework wrappers in the MVP.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `ls examples/` includes html, react, vue, svelte
- [x] the SDK README links each example

## Evidence & Notes

- Added examples in `jovrion/veylqo-js/examples/`:
  - `html/index.html`
  - `react/README.md`
  - `vue/README.md`
  - `svelte/README.md`
- Updated `jovrion/veylqo-js/README.md` with links to each example and a React `< 19` note (custom event binding via `ref` + `addEventListener`).

Verification output (2026-11-12):

```bash
$ ls examples/
html
react
svelte
vue

$ pnpm test
> veylqo-js@ test /Users/jarias/code/veylqo-js
> pnpm -r test
...
packages/element test: ℹ pass 1
packages/cdn test: ℹ pass 3
packages/core test: ℹ pass 14

$ pnpm -r build
Scope: 3 of 4 workspace projects
packages/cdn build$ node ./scripts/build.mjs
packages/cdn build: Done
```
