---
id: VYQ-22
type: task
title: "[go] RecordClick, DeleteLink, GetLink, LinkStats"
status: "done"
priority: "major"
area: "go"
epic: VYQ-3
depends_on: [VYQ-17, VYQ-18]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-22 — [go] RecordClick, DeleteLink, GetLink, LinkStats

## Scope

Command `RecordClick` (does not block the caller: accepts an async port). `DeleteLink`. Queries `GetLink` and `LinkStats` (counts, no PII).

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/app/...` green for these handlers
- [x] `RecordClick` does not wait for the click repository to finish

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/app/... -count=1
ok  github.com/jovrion/veylqo/internal/links/app/command
ok  github.com/jovrion/veylqo/internal/links/app/query
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/common/errors
ok  github.com/jovrion/veylqo/internal/links/app/command
ok  github.com/jovrion/veylqo/internal/links/app/query
ok  github.com/jovrion/veylqo/internal/links/domain/link
go test -race ./internal/links/app/... -count=1 -run 'RecordClickDoesNotWait|RecordClick$|DeleteLink$|GetLink$|LinkStats$'
ok  github.com/jovrion/veylqo/internal/links/app/command
ok  github.com/jovrion/veylqo/internal/links/app/query
```

`go test ./internal/links/app/... -count=1 -v -run 'RecordClick|DeleteLink|GetLink|LinkStats'` passed. `TestRecordClickDoesNotWaitForClickRepository` returns from `Handle` while `ClickRepository.Add` is blocked on an unreleased channel, then the store finishes only after release. The handler calls `ClickRecorder.Enqueue` and does not take a click repository. A cancelled context is not enqueued. Invalid, reserved, and UUID codes are `incorrect-input` and are not enqueued.

`DeleteLink` uses `Repository.Update` and `Link.Delete`. Missing, invalid, and UUID codes are `not-found` (a UUID is not looked up). A second delete is `gone` and keeps the first timestamp. An expired link can still be deleted. A zero clock is `incorrect-input` and does not update. Store errors stay unsluged. Error text does not include the destination.

`GetLink` returns the stored link, including expired and deleted rows, and omits the internal UUID. It does not record a click. `LinkStats` returns `Clicks` only. A missing link is `not-found` and does not call `Count`. A deleted link still has a count. A negative count is not a result. Neither query imports `net/http` or `database/sql`.

Wiring into `app.Application` is VYQ-23. The batcher is VYQ-28. HTTP is not this task.
