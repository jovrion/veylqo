---
id: VYQ-8
type: task
title: "[setup] work/ tracking and OpenCode config"
status: "done"
priority: "critical"
area: "setup"
epic: VYQ-1
depends_on: []
blocked_reason: null
updated: 2026-11-09
---

# VYQ-8 — [setup] work/ tracking and OpenCode config

## Scope

Install the local board (`work/`, `scripts/work.py`) and OpenCode config (agents, slash commands, permissions) from the `fraud-detection-platform` model, adapted to IDs `VYQ-<n>`, areas `setup|docs|go|contract|js|ops`, and phases F0–F6. English throughout. Native OpenCode V2 config (`permissions`, `agents`, no deprecated keys).

Governs: `WORKFLOW.md`, `work/README.md`, `AGENTS.md`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `make work-validate` exits 0
- [x] `make work-next` prints a `VYQ-<n>` recommendation
- [x] `.opencode/agents/{planner,builder-go,builder-js,architecture-guardian,debugger}.md` exist
- [x] `.opencode/commands/{work-next,work-start,work-done,review,fix-bug,write-tests,generate-pr}.md` exist
- [x] `scripts/work.py` rejects IDs that do not match `VYQ-<n>`
- [x] `.opencode/opencode.jsonc` uses V2 keys (`permissions`, `agents`) and has no `permission`, `agent`, `provider`, or `small_model`

## Evidence & Notes

Created in the bootstrap session. Remote `git@github.com-unisimon:jovrion/veylqo.git`. Board regenerated with `make work-board` and validated with `make work-validate`. Docs, agents, and commands translated to English. OpenCode config migrated to native V2.
