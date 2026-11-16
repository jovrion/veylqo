---
description: Generate a PR description from the current changes
agent: general
---
Generate the PR description for the current changes.
Diff: !`if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then git diff HEAD; else for r in veylqo veylqo-js; do [ -d "$r/.git" ] && printf '=== %s ===\n' "$r" && git -C "$r" diff HEAD; done; fi`

Include: change summary, related `veylqo/work/` task (ID `VYQ-<n>`), architecture or security decisions, conformance or gaps versus `AGENTS.md` and the ADRs in `veylqo/docs/adr/`, and verification commands that were run.
Do not claim checks that do not appear in the diff or in the task evidence.
