---
id: VYQ-42
type: task
title: "[docs] Self-hosted operations README"
status: "done"
priority: "minor"
area: "docs"
epic: VYQ-6
depends_on: [VYQ-38]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-42 — [docs] Self-hosted operations README

## Scope

How to run the binary and Compose, environment variables, how to put a short domain in front, SQLite limits, approximate-analytics note. Minimal SECURITY.md and CONTRIBUTING.md.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `grep -q VEYLQO_ALLOWED_HOSTS README.md`
- [x] `test -f SECURITY.md -a -f CONTRIBUTING.md`

## Evidence & Notes

2026-11-12. `README.md` documents the binary, Compose, the environment table (including `VEYLQO_ALLOWED_HOSTS`), a reverse proxy in front of `:8080`, SQLite as one writer, and approximate click counts. `VEYLQO_BASE_URL` is documented as unread. `SECURITY.md` and `CONTRIBUTING.md` are present. No `.env` was committed. No Go code changed.

```
grep -q VEYLQO_ALLOWED_HOSTS README.md && echo 'grep: ok'
grep: ok
test -f SECURITY.md -a -f CONTRIBUTING.md && echo 'files: ok'
files: ok
gofmt -s -w .
go vet ./...
go test ./...
go test ./internal/archtest/...
make work-validate
work: OK (48 items, 0 warnings)
```
