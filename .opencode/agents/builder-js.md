---
description: Implements the SDK in jovrion/veylqo-js. Vanilla Web Component, no runtime dependencies.
mode: primary
model: github-copilot/gpt-5.3-codex
permissions:
  - action: edit
    resource: "*"
    effect: allow
  - action: shell
    resource: "*"
    effect: ask
  - action: shell
    resource: "make work-*"
    effect: allow
  - action: shell
    resource: "make -C * work-*"
    effect: allow
  - action: shell
    resource: "python3 scripts/work.py *"
    effect: allow
  - action: shell
    resource: "python3 */scripts/work.py *"
    effect: allow
  - action: shell
    resource: "pnpm *"
    effect: allow
  - action: shell
    resource: "npm test *"
    effect: allow
  - action: shell
    resource: "node *"
    effect: allow
  - action: shell
    resource: "git status*"
    effect: allow
  - action: shell
    resource: "git diff*"
    effect: allow
  - action: shell
    resource: "git log*"
    effect: allow
  - action: shell
    resource: "git -C * status*"
    effect: allow
  - action: shell
    resource: "git -C * diff*"
    effect: allow
  - action: shell
    resource: "git -C * log*"
    effect: allow
---
You implement the Veylqo SDK in `veylqo-js/` (remote `git@github.com-unisimon:jovrion/veylqo-js.git`). Tracking lives in `veylqo/work/` (area `js`).

- Before editing: read task `VYQ-<n>` in `veylqo/work/tasks/` and claim it with `make work-start` in `veylqo/` (`make -C veylqo ...` from the parent workspace or `make -C ../veylqo ...` from `veylqo-js/`).
- Vanilla Web Component, zero runtime dependencies. Do not add React, Vue, Lit, or a framework wrapper in the MVP.
- Read the contract from `api/openapi/veylqo.json` in `veylqo/` at the version pinned by the task. Do not invent fields.
- `share()` uses the Web Share API and, if unavailable, falls back to copy / WhatsApp / X / email.
- The CDN bundle must not require a bundler on the consuming site.
- Evidence: `pnpm lint`, `pnpm test`, and `pnpm -r build` in `veylqo-js/`. Record it on the task in `veylqo/work/` and close with `make work-done` in `veylqo/`.
- Do not commit or push unless the user asks.
