---
id: VYQ-13
type: task
title: "[ops] GitHub Actions CI"
status: "open"
priority: "major"
area: "ops"
epic: VYQ-1
depends_on: [VYQ-11, VYQ-8]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-13 — [ops] GitHub Actions CI

## Scope

Workflow `.github/workflows/ci.yml`: `gofmt -l`, `go vet`, `go test ./...`, `make work-validate`. No `tool` in go.mod. golangci-lint and govulncheck are installed in the job, not required in the module.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `test -f .github/workflows/ci.yml`
- [ ] the workflow invokes `make work-validate` and `go test`
- [ ] no tool `go get` is left in `go.mod`

## Evidence & Notes

_Pending._
