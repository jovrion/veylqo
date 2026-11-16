---
description: Claim a VYQ-<n> task (or resume the current one) and start implementing
---
Requested task: $1

Status and claim attempt:
!`d=.; [ -f veylqo/Makefile ] && d=veylqo; [ -f ../veylqo/Makefile ] && d=../veylqo; if [ -n "$1" ]; then make -C "$d" work-start ID="$1" || make -C "$d" work-next; else make -C "$d" work-next; fi`

1. Find and read the task file under `work/tasks/` in `veylqo/` (from ID `$1` or the `in-progress` task).
2. Review its `## Scope` and every `## Verification` item.
3. Confirm the requested task is `in-progress`. If the claim fails because another task is in progress, do not implement a different one automatically.
4. Read the relevant `AGENTS.md` (`veylqo/AGENTS.md` and/or `veylqo-js/AGENTS.md`) and the ADR named by the task in `veylqo/docs/adr/`. If the `area` does not match your role, say so before editing. Area `js` is implemented in `veylqo-js/`.
5. Implement in small, verifiable steps. Collect real evidence for each checkbox. Do not tick evidence you did not run.
6. Do not commit or push unless the user asks.
