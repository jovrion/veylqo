---
id: VYQ-48
type: task
title: "[go] fix click batcher nil context test"
status: "done"
priority: "major"
area: "go"
epic: VYQ-4
depends_on: [VYQ-28]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-48 — [go] fix click batcher nil context test

## Scope

Restore `TestClickBatcherEnqueueDoesNotAcceptRejectedClicks` so a nil
context is rejected, not accepted as `context.Background()`. ADR-0009
still holds: a rejected click is not queued and is not a drop. Shut the
batcher down if an assertion fails, so `testing/synctest` does not report
a deadlock after the real failure.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/adapters/ -run TestClickBatcherEnqueueDoesNotAcceptRejectedClicks -count=1` green
- [x] `gofmt -s -w .`, `go vet ./...`, and `go test ./...` green

## Evidence & Notes

2026-11-11:

`TestClickBatcherEnqueueDoesNotAcceptRejectedClicks` passed `context.Background()` where it meant a nil context (`2a44c36`). `Enqueue` accepted it, `t.Fatal` left `run` blocked, and `testing/synctest` reported a deadlock. The check now passes a nil `context.Context` variable so staticcheck SA1012 does not rewrite the call. `defer shutdownBatcher` drains the bubble if an assertion fails.

```
gofmt -s -w .
go vet ./...
go test ./internal/links/adapters/ -run TestClickBatcherEnqueueDoesNotAcceptRejectedClicks -count=1 -v
--- PASS: TestClickBatcherEnqueueDoesNotAcceptRejectedClicks (0.00s)
ok  github.com/jovrion/veylqo/internal/links/adapters
go test ./...
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/common/decorator
ok  github.com/jovrion/veylqo/internal/common/errors
ok  github.com/jovrion/veylqo/internal/links/adapters
ok  github.com/jovrion/veylqo/internal/links/app
ok  github.com/jovrion/veylqo/internal/links/app/command
ok  github.com/jovrion/veylqo/internal/links/app/query
ok  github.com/jovrion/veylqo/internal/links/domain/link
ok  github.com/jovrion/veylqo/internal/migrate
go test ./internal/archtest/...
ok  github.com/jovrion/veylqo/internal/archtest
golangci-lint run ./internal/links/adapters/...
0 issues.
```
