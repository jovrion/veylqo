---
id: VYQ-11
type: task
title: "[setup] go.mod 1.27, MIT LICENSE, and build Makefile"
status: "open"
priority: "critical"
area: "setup"
epic: VYQ-1
depends_on: [VYQ-8]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-11 — [setup] go.mod 1.27, MIT LICENSE, and build Makefile

## Scope

Module `github.com/jovrion/veylqo`, `go 1.27`, MIT LICENSE. Makefile gains `build`, `test`, `vet`, `fmt` in addition to `work-*`. No driver `require` yet (they arrive with the adapters). Short README: what Veylqo is, how to clone, how to run the board.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `grep '^go 1.27' go.mod`
- [ ] `test -f LICENSE` and it contains MIT
- [ ] `make test` and `make vet` exist and do not fail on the current tree

## Evidence & Notes

_Pending._
