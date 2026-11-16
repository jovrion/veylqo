---
description: Verify evidence, tick Verification, and close the active task
---
Task to close: $1
Current status:
!`d=.; [ -f veylqo/Makefile ] && d=veylqo; [ -f ../veylqo/Makefile ] && d=../veylqo; make -C "$d" work-next`

To close the task in progress:

1. Read the task file under `work/tasks/` in `veylqo/`.
2. Run the commands for each `## Verification` item and the `AGENTS.md` checks that apply (`gofmt`, `go vet`, `go test` in `veylqo/`, or `pnpm lint`, `pnpm test`, `pnpm -r build` in `veylqo-js/` for `js` tasks, plus `make work-validate` in `veylqo/`). Do not tick evidence you did not run.
3. Only if they all pass, update the file ticking `- [x]` and document evidence under `## Evidence & Notes`.
4. Run `make work-done ID=<ID>` and `make work-validate` in `veylqo/`.
5. Show `make work-next` in `veylqo/` so the next item is visible.
