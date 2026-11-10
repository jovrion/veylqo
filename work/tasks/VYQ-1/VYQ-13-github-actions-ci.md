---
id: VYQ-13
type: task
title: "[ops] GitHub Actions CI"
status: "done"
priority: "major"
area: "ops"
epic: VYQ-1
depends_on: [VYQ-11, VYQ-8]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-13 — [ops] GitHub Actions CI

## Scope

Workflow `.github/workflows/ci.yml`: `gofmt -l`, `go vet`, `go test ./...`, `make work-validate`. No `tool` in go.mod. golangci-lint and govulncheck are installed in the job, not required in the module.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `test -f .github/workflows/ci.yml`
- [x] the workflow invokes `make work-validate` and `go test`
- [x] no tool `go get` is left in `go.mod`

## Evidence & Notes

2026-11-10. Workflow `.github/workflows/ci.yml` on `ubuntu-latest` with Go from `go.mod` (`go 1.27`). Steps: `gofmt -l`, `go vet ./...`, `go test ./...`, `make work-validate`. `golangci-lint` v2.15.0 is the binary install script. `govulncheck` v1.9.0 is `go install pkg@version` from a temp directory. Neither writes `go.mod`. No `tool` directive (ADR-0003).

`golangci-lint run` failed once on an unchecked `file.Close` in `internal/archtest/rules.go`. The close error is now returned when the scan itself succeeded, so the job is not red on the current tree.

```
test -f .github/workflows/ci.yml
# exit 0

grep -n -E 'make work-validate|go test' .github/workflows/ci.yml
# 46:        run: go test ./...
# 49:        run: make work-validate

grep -nE '^(tool |require )' go.mod
# no match; file is only `module github.com/jovrion/veylqo` and `go 1.27`
# no go.sum

gofmt -l .
# empty, exit 0

go vet ./...
go test ./...
go test ./internal/archtest/...
# ok  github.com/jovrion/veylqo/internal/archtest

golangci-lint run
# golangci-lint has version 2.15.0 built with go1.27.0
# 0 issues.

govulncheck ./...
# govulncheck@v1.9.0 built with go1.27.3
# No vulnerabilities found.
# go.mod unchanged after both installs

make work-validate
# work: OK (47 items, 0 warnings)
```
