# Work tracking — Veylqo

This directory is the **source of truth for progress**. There is no
external tracker.

Design lives in `docs/architecture.md` and `docs/adr/`; here we only
record what is being done and in which order. Tasks with area `js` are
implemented in `jovrion/veylqo-js`, but they are closed here.

## Layout

```
work/
├── README.md            this file — the rules
├── NEXT-ID              next free ID; incremented when an item is created
├── BOARD.md             generated index (make work-board); do not edit by hand
├── board.html           visual board (make work-view); git-ignored
├── _templates/          epic and task templates
├── epics/               one file per epic (one phase F0–F6)
└── tasks/<epic-id>/     one file per task, grouped by epic
```

## Identifiers

- Format `VYQ-<n>`. The next free number is in `NEXT-ID`.
- An ID is permanent; numbers are not reused.
- File name: `VYQ-<n>-<short-kebab-title>.md`.

## Statuses

```
open ──► in-progress ──► done
  └────► blocked (requires blocked_reason)
```

| Status        | Meaning                                          |
| ------------- | ------------------------------------------------ |
| `open`        | Backlog, not started                             |
| `in-progress` | Active implementation (at most 1 at a time)      |
| `blocked`     | Cannot proceed; `blocked_reason` is required     |
| `done`        | Verified with real evidence                      |

## Transition rules (enforced by `make work-validate`)

1. **`open → in-progress`**: every dependency in `depends_on` must be
   `done`. Only **1 task in progress** at a time.
2. **`in-progress → done`**: every checkbox in `## Verification` must
   be ticked (`- [x]`), each with re-runnable evidence (a command, a
   test, or a file path).
3. **Anything → `blocked`**: requires a non-empty `blocked_reason`.

An epic's status is derived: `open` while no task has started,
`in-progress` if any left `open`, `done` when all are `done`.

A task's `depends_on` points at **tasks**, not epics: an epic has no
`status` of its own and would block start forever.

## Front matter

### Task

```yaml
id: VYQ-13
type: task
title: "[go] Value objects Code and Destination"
status: open              # open|in-progress|blocked|done
priority: critical        # critical|major|normal|minor
area: go                  # setup|docs|go|contract|js|ops
epic: VYQ-2
depends_on: [VYQ-12]      # hard edges only: without the other, this cannot start
blocked_reason: null
updated: 2026-11-09
```

### Epic

```yaml
id: VYQ-1
type: epic
title: "F0 — Governance and repo foundation"
phase: F0                 # F0|F1|F2|F3|F4|F5|F6
depends_on: []
updated: 2026-11-09
```

`depends_on` is declared only on the dependent; the script derives the
inverse `blocks` for the board. The graph must be acyclic.

## Commands

```bash
make work-validate   # schema, links, cycles, WIP limit, fresh board (fails if stale)
make work-board      # regenerate work/BOARD.md
make work-view       # generate work/board.html and open it (NOOPEN=1 only generates)
make work-next       # recommend the next action (phase, then priority)
make work-start ID=VYQ-10   # claim an open, ready task
make work-done  ID=VYQ-10   # close if Verification is complete
make work-new EPIC=VYQ-2 AREA=go TITLE="..."   # new task with a free ID
```

`make work-next` ranks by phase. To pull a later-phase task whose
dependencies are already `done`, use `make work-start` with its ID.

## Working agreement

1. Open the task file and read it fully.
2. `make work-start ID=...` (rejects if another task is in progress or
   a dependency is unfinished).
3. Implement in small, verifiable steps.
4. Tick each `## Verification` checkbox only with real evidence.
5. `make work-done ID=...` before closing. The command regenerates the board.
