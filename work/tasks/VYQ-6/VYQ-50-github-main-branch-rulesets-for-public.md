---
id: VYQ-50
type: task
title: "[ops] GitHub main branch rulesets for public repos"
status: "done"
priority: "major"
area: "ops"
epic: VYQ-6
depends_on: [VYQ-13, VYQ-49]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-50 — [ops] GitHub main branch rulesets for public repos

## Scope

Active `protect-main` rulesets on `jovrion/veylqo` and `jovrion/veylqo-js`,
applied with `gh` from `.github/rulesets/protect-main.json` in each repo.
No bypass actors. Direct push, force push, and deletion of the default
branch are rejected. A pull request is required. Approving reviews are
not required. Required checks: `test` and `postgres` here, `test` in the
SDK repo. Squash or rebase only.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `gh api repos/jovrion/veylqo/rulesets --jq '.[] | select(.name=="protect-main") | .enforcement'` prints `active`
- [x] `gh api repos/jovrion/veylqo-js/rulesets --jq '.[] | select(.name=="protect-main") | .enforcement'` prints `active`
- [x] both rulesets list `pull_request`, `non_fast_forward`, `deletion`, and `required_status_checks`, with zero bypass actors

## Evidence & Notes

2026-11-12. Applied with `gh api --method POST` as `joeariasunisimon` (admin on both public repos). Create responses: `current_user_can_bypass` is `never`. Ruleset ids: `jovrion/veylqo` 27157485, `jovrion/veylqo-js` 27157487. Desired state is the JSON file in each repo; a later edit needs `PUT` on that id, not a second `POST`.

```
gh api repos/jovrion/veylqo/rulesets --jq '.[] | select(.name=="protect-main") | .enforcement'
# "active"

gh api repos/jovrion/veylqo-js/rulesets --jq '.[] | select(.name=="protect-main") | .enforcement'
# "active"

gh api repos/jovrion/veylqo/rulesets/27157485 --jq '{bypass:(.bypass_actors|length),checks:[.rules[]|select(.type=="required_status_checks").parameters.required_status_checks[].context],reviews:.rules[]|select(.type=="pull_request").parameters.required_approving_review_count}'
# {"bypass":0,"checks":["test","postgres"],"reviews":0}

gh api repos/jovrion/veylqo-js/rulesets/27157487 --jq '{bypass:(.bypass_actors|length),checks:[.rules[]|select(.type=="required_status_checks").parameters.required_status_checks[].context]}'
# {"bypass":0,"checks":["test"]}

gh api repos/jovrion/veylqo/rules/branches/main --jq '[.[].type]'
# ["deletion","non_fast_forward","required_linear_history","pull_request","required_status_checks"]

gh api repos/jovrion/veylqo-js/rules/branches/main --jq '[.[].type]'
# ["deletion","non_fast_forward","required_linear_history","pull_request","required_status_checks"]
```
