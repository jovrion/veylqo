---
id: VYQ-11
type: task
title: "[setup] go.mod 1.27, MIT LICENSE, and build Makefile"
status: "done"
priority: "critical"
area: "setup"
epic: VYQ-1
depends_on: [VYQ-8]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-11 — [setup] go.mod 1.27, MIT LICENSE, and build Makefile

## Scope

Module `github.com/jovrion/veylqo`, `go 1.27`, MIT LICENSE. Makefile gains `build`, `test`, `vet`, `fmt` in addition to `work-*`. No driver `require` yet (they arrive with the adapters). Short README: what Veylqo is, how to clone, how to run the board.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `grep '^go 1.27' go.mod`
- [x] `test -f LICENSE` and it contains MIT
- [x] `make test` and `make vet` exist and do not fail on the current tree

## Evidence & Notes

2026-11-10. Module `github.com/jovrion/veylqo`, no `require` and no `tool` directive. Drivers stay out until the adapter tasks (ADR-0003).

```
grep '^go 1.27' go.mod
# go 1.27
# exit 0

test -f LICENSE && grep -n MIT LICENSE
# 1:MIT License
# exit 0

make test
# test: no Go packages yet
# exit 0

make vet
# vet: no Go packages yet
# exit 0
```

`go vet ./...` and `go test ./...` exit 1 with "matched no packages" until the first package exists. `make vet` and `make test` treat only that case as a no-op; any other `go list` failure still fails the target. `make fmt` and `make build` also exit 0 on this tree.
