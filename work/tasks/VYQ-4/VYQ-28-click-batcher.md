---
id: VYQ-28
type: task
title: "[go] click_batcher"
status: "done"
priority: "major"
area: "go"
epic: VYQ-4
depends_on: [VYQ-22]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-28 — [go] click_batcher

## Scope

Bounded channel; flush every N events or T seconds; drain on shutdown. Full channel: drop the click and count it in a metric. Tests with `testing/synctest`. Analytics are approximate.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/adapters/ -run Batcher` green
- [x] a shutdown with pending events persists them

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/adapters/ -run Batcher
ok  github.com/jovrion/veylqo/internal/links/adapters
go test -race ./internal/links/adapters/ -count=1 -run Batcher
ok  github.com/jovrion/veylqo/internal/links/adapters
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/common/decorator
ok  github.com/jovrion/veylqo/internal/common/errors
ok  github.com/jovrion/veylqo/internal/links/adapters
ok  github.com/jovrion/veylqo/internal/links/app
ok  github.com/jovrion/veylqo/internal/links/app/command
ok  github.com/jovrion/veylqo/internal/links/app/query
ok  github.com/jovrion/veylqo/internal/links/domain/link
ok  github.com/jovrion/veylqo/internal/migrate
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
```

`TestClickBatcherShutdownPersistsPending` enqueues two clicks for `Ab3xyZ9` and one for `zzzzzzz` with batch 100 and an hour interval. After `synctest.Wait` the store is still empty. `Shutdown` then persists 2 and 1, coalesced into one `Add` per code.

`TestClickBatcherFlushesEveryN` writes only when the third event arrives, as one `Add` of n=3. `TestClickBatcherFlushesOnInterval` does not write at T-1ns and writes at T. `TestClickBatcherDropsWhenFull` returns from `Enqueue` while `Add` is blocked, fills a capacity-1 channel, drops the next click, and `Dropped` is 1. The dropped code is not stored. A failed flush is retried on the next interval; a failed shutdown flush returns the store error and does not report the count as durable.

Analytics stay approximate: a full channel or a crash before flush loses clicks. No IP, token, or destination is accepted. Wiring into `cmd` and `Server.Shutdown` is VYQ-38. Not committed.
