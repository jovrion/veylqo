---
id: VYQ-15
type: task
title: "[go] Destination value object"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-2
depends_on: [VYQ-12]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-15 — [go] Destination value object

## Scope

Destination URL: `http`/`https` only; reject `javascript:`, `data:`, and, when configured, private/loopback hosts via `net/netip`. Normalize (lowercase host, no fragment). Table tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/domain/link/ -run Destination` green
- [x] `javascript:alert(1)` and `http://127.0.0.1/` (with blocking on) fail

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/domain/link/ -run Destination -count=1
ok  github.com/jovrion/veylqo/internal/links/domain/link
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/links/domain/link
```

`go test ./internal/links/domain/link/ -run 'Destination/(javascript$|loopback_blocking_on$)' -count=1 -v` passed `TestNewDestination/javascript` and `TestNewDestination/loopback_blocking_on`. Those cases expect `ErrInvalidDestination` with reasons `scheme` and `private`. The error text does not include the URL.

Blocking on is the zero `DestinationConfig`. `AllowPrivateHosts` opts out of the address block only; `javascript:` still fails. Classification uses `net/netip` after `Unmap` (loopback, private, link-local, unspecified), plus the names `localhost` and `*.localhost`. No DNS lookup and no fetch. Tracking-parameter stripping stays in VYQ-16. The allowlist stays in the shorten policy (VYQ-20). `netip.IsPrivate` does not include `100.64.0.0/10`, so this check does not either.
