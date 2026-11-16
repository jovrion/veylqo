---
id: VYQ-57
type: task
title: "[ops] Private multi-tenant operations"
status: "open"
priority: "major"
area: "ops"
epic: VYQ-51
depends_on: [VYQ-56]
blocked_reason: null
updated: 2026-11-15
---

# VYQ-57 — [ops] Private multi-tenant operations

## Scope

Operator-facing only. Do not change redirect status, domain rules, or
the spec. Read ADR-0013.

- Make `jovrion/veylqo` private. Leave `jovrion/veylqo-js` as the
  distributable SDK repo. Do not print a publishable key or an admin
  token in the README, in CI logs, or in a committed env file.
- Update the operations README so an operator can set
  `VEYLQO_PUBLIC_BASE_URL`, the default publishable key, and
  `VEYLQO_ALLOWED_HOSTS`, and can add another site with the admin token.
  State that share and click counts are approximate, that a crash before
  flush drops events, and that stats are not a visit log.
- State that the public short URL is
  `https://<public-base>/{prefix}/{code}` on the Veylqo host. The
  customer site does not have to proxy `/d/` or any other prefix.
- `main` stays protected. A visibility change is a repository setting,
  not a direct push to `main`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `gh repo view jovrion/veylqo --json visibility --jq .visibility` prints `PRIVATE`
- [ ] the operations README names `VEYLQO_PUBLIC_BASE_URL` and states that share and click analytics are approximate
- [ ] `git grep -n 'VEYLQO_ADMIN_TOKEN=.' -- ':!*.md' ':!docs/adr'` finds no committed token assignment

## Evidence & Notes
