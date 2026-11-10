---
description: Verify evidence, tick Verification, and close the active task
---
Task to close: $1
Current status:
!`make work-next`

To close the task in progress:

1. Read the task file under `work/tasks/`.
2. Run the commands for each `## Verification` item and the `AGENTS.md` checks that apply (`gofmt`, `go vet`, `go test`, `make work-validate`). Do not tick evidence you did not run.
3. Only if they all pass, update the file ticking `- [x]` and document evidence under `## Evidence & Notes`.
4. Run `make work-done ID=<ID>` and `make work-validate`.
5. Show `make work-next` so the next item is visible.
