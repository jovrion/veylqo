# Workflow — how this repository is worked

Progress is tracked with a **local board** in `work/` (IDs `VYQ-<n>`).
Markdown files are the source of truth. Views (`BOARD.md`, `board.html`)
are generated with Python stdlib (`scripts/work.py`). There is no
external tracker.

Design lives in `docs/architecture.md` and `docs/adr/`. This file only
records how work is claimed and closed. The SDK is implemented in
`jovrion/veylqo-js`, but its tasks (area `js`) are tracked here.

## Task cycle

In the terminal:

```
make work-next              # what is next? (dependencies and WIP=1)
make work-start ID=VYQ-10   # claim: open → in-progress (at most 1 at a time)
... implement in small, verifiable steps ...
make work-done  ID=VYQ-10   # close: every Verification box must be ticked
make work-validate          # schema, links, cycles, and a fresh board
```

In OpenCode, the same cycle uses slash commands (`.opencode/commands/`):

```
/work-next            # board status + summary of the active or recommended task
/work-start VYQ-10    # claim (or resume if already in-progress) and start implementing
/work-done VYQ-10     # verify evidence, tick checkboxes, and close the task
```

`/work-start` resumes a claimed task: if `make work-start` fails because
it is already `in-progress`, the command reads the board and continues
that task. It does not start a different one on its own.

- **Statuses**: `open → in-progress → done`, or `→ blocked` (requires
  `blocked_reason`).
- **Dependencies**: a task may start only when every `depends_on` entry
  is `done`. The graph is acyclic and validated.
  `make work-next` ranks by phase and priority; `make work-start`
  can pull a later-phase task forward if its hard dependencies are
  `done`.
- **Evidence**: tick a `## Verification` checkbox only with something
  re-runnable (a command, a test, a path). `work-done` refuses to close
  if any box is open.
- **Fresh board**: `make work-validate` fails if `work/BOARD.md` does
  not match the files. Regenerate with `make work-board`.

## Board commands

| Command | What it does |
| --- | --- |
| `make work-validate` | Schema, links, cycles, WIP limit, and board freshness |
| `make work-board` | Regenerates `work/BOARD.md` |
| `make work-view` | Builds `work/board.html` (Map + Kanban) and opens it; `NOOPEN=1` only builds |
| `make work-next` | Recommends the next action |
| `make work-start ID=VYQ-<n>` | Claims a ready task |
| `make work-done ID=VYQ-<n>` | Closes if Verification is complete |
| `make work-new EPIC=VYQ-2 AREA=go TITLE="..."` | New task with the next free ID |

## OpenCode flow (commands and agents)

Slash commands live in `.opencode/commands/` and reload without restarting OpenCode:

| Command | What it does |
| --- | --- |
| `/work-next` | Board status and a summary of the active or recommended task |
| `/work-start VYQ-<n>` | Claims (or resumes) the task and starts implementation |
| `/work-done VYQ-<n>` | Verifies evidence, ticks Verification, and closes the task |
| `/review` | Architecture audit of the diff (`architecture-guardian`) |
| `/fix-bug` | Diagnose and fix a bug (`builder-go`) |
| `/write-tests` | Tests for a package or file (`builder-go`) |
| `/generate-pr` | PR description from the current diff (`general`) |

Suggested agent by task `area`:

- `go`, `contract`, `setup`, `ops`, `docs` → `builder-go`
- `js` → `builder-js` (code lives in `jovrion/veylqo-js`)
- Design / trade-offs / plans → `planner` (read-only)
- Audit a change → `/review` (`architecture-guardian`)
- Incidents → `debugger` (read-only)

`make work-*` permissions are `allow` in `.opencode/opencode.jsonc` and
on the `builder-*` agents, so the board flow does not prompt on every
board command.

## Repository map

| File / directory | Role |
| --- | --- |
| `work/README.md` | Tracking rules (statuses, transitions, front matter) |
| `work/BOARD.md` | Generated board — progress by phase and area |
| `work/epics/`, `work/tasks/` | Epics (F0–F6) and `VYQ-<n>` tasks |
| `AGENTS.md` | Agent instructions (architecture, verification, limits) |
| `CLAUDE.md` | Imports `AGENTS.md`; does not duplicate rules |
| `docs/architecture.md` | Design: layers, flows, security (VYQ-10) |
| `docs/adr/` | Accepted decisions, MADR format |

## Hard rules

1. **No real secrets**: never commit `.env`. Do not log full destination
   URLs with query strings, IPs, or tokens.
2. **Stdlib first**: no new `require` in `go.mod` without an ADR.
   The only planned exceptions are the two database drivers, imported
   only in `cmd/veylqo`.
3. **The contract wins**: an API change starts in
   `api/openapi/veylqo.json`. There are no generated files to edit by hand.
4. **Close with evidence**: every task ends with commands or tests that
   prove it. Do not tick a checkbox you did not run.
5. **Do not claim what the board has not marked `done`**: a task is
   intent, not an assertion that the code already exists.

Operational detail is in `work/README.md`. Architecture rules are in
`AGENTS.md`.
