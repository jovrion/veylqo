---
description: Show the in-progress task or the next recommended board item
---
Current tracking status:
!`make work-next`

1. If a task is `in-progress` or recommended (`Recommended: VYQ-<n>`), open and read its file under `work/tasks/`.
2. Summarize its `Scope`, each `## Verification` item, and the `area`.
   - `go`, `contract`, `setup`, `ops`, `docs` → `builder-go`
   - `js` → `builder-js` (repo `jovrion/veylqo-js`)
   - Design without edits → `planner`
3. Contrast the scope with `AGENTS.md` and the ADR named by the task. Propose small, verifiable steps, or say whether it should be claimed with `/work-start VYQ-<n>`.
4. Do not implement anything in this command.
