---
description: Show the in-progress task or the next recommended board item
---
Current tracking status:
!`d=.; [ -f veylqo/Makefile ] && d=veylqo; [ -f ../veylqo/Makefile ] && d=../veylqo; make -C "$d" work-next`

1. If a task is `in-progress` or recommended (`Recommended: VYQ-<n>`), open and read its file under `work/tasks/` in `veylqo/`.
2. Summarize its `Scope`, each `## Verification` item, and the `area`.
   - `go`, `contract`, `setup`, `ops`, `docs` → `builder-go` (repo `veylqo/`)
   - `js` → `builder-js` (repo `veylqo-js/`)
   - Design without edits → `planner`
3. Contrast the scope with the relevant `AGENTS.md` (`veylqo/AGENTS.md` and/or `veylqo-js/AGENTS.md`) and the ADR named by the task in `veylqo/docs/adr/`. Propose small, verifiable steps, or say whether it should be claimed with `/work-start VYQ-<n>`.
4. Do not implement anything in this command.
