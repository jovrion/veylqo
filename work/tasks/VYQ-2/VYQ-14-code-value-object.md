---
id: VYQ-14
type: task
title: "[go] Code value object"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-2
depends_on: [VYQ-12]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-14 — [go] Code value object

## Scope

`internal/links/domain/link/code.go`: random 7-character base62 codes; custom slugs 3–64, regex `[a-zA-Z0-9_-]+`; reserved `api`, `healthz`, `readyz`, `metrics`, `debug`, `static`. Validating constructor. Table tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/domain/link/ -run Code` green
- [x] slug `api` or `healthz` is an error
- [x] `go test ./internal/archtest/...` stays green

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/domain/link/ -run Code -count=1
ok  github.com/jovrion/veylqo/internal/links/domain/link
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/links/domain/link
```

`go test ./internal/links/domain/link/ -run 'Code/(reserved_api|reserved_healthz)$' -count=1 -v` passed `TestNewCode/reserved_api` and `TestNewCode/reserved_healthz`. Those cases expect `NewCode` to return `ErrInvalidCode` with reason `reserved`. Case-folded forms (`API`, `HealthZ`, and the other reserved names) are in the same table.

`NewCode` validates only. Unbiased `crypto/rand` generation stays in VYQ-30. Alphabet is `Base62Alphabet` in the domain so adapters do not define a second one (ADR-0006).
