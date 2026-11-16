---
description: Architecture review of the current change
agent: architecture-guardian
subagent: true
---
Review the current changes.
Diff: !`if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then git diff HEAD; else for r in veylqo veylqo-js; do [ -d "$r/.git" ] && printf '=== %s ===\n' "$r" && git -C "$r" diff HEAD; done; fi`
Contrast with `AGENTS.md` (`veylqo/AGENTS.md` and/or `veylqo-js/AGENTS.md`) and the ADRs in `veylqo/docs/adr/`: layers, stdlib/zero-runtime-dependency policy, OpenAPI contract, 302 redirects, public-shortening abuse, and logs. Call out missing tests.
Additional focus: $ARGUMENTS
