---
id: VYQ-21
type: task
title: "[go] query ResolveLink"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-3
depends_on: [VYQ-17, VYQ-18]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-21 — [go] query ResolveLink

## Scope

Return the destination or `not-found` / `gone`. Does not record the click (that is `RecordClick`). Does not import HTTP.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/app/query/ -run Resolve` green
- [x] an expired link is `gone`, a missing one `not-found`

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/app/query/ -run Resolve -count=1
ok  github.com/jovrion/veylqo/internal/links/app/query
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/common/errors
ok  github.com/jovrion/veylqo/internal/links/app/query
ok  github.com/jovrion/veylqo/internal/links/domain/link
```

`go test ./internal/links/app/query/ -run Resolve -count=1 -v` passed. `TestResolveLink/expired_is_gone` returns slug `gone` and no destination. `TestResolveLink/missing_is_not-found` returns slug `not-found` for `link.NotFoundError`. A deleted row is also `gone` (ADR-0007). A UUID is `not-found` and is not looked up. The handler calls `Get` only; the fake records zero writes. Production files under `internal/links/app/query` do not import `net/http` or `database/sql`. Error text does not include the stored URL or its query. HTTP status mapping stays in ports. Wiring into `app.Application` is VYQ-23. The resolve cache is VYQ-29.
