---
id: VYQ-32
type: task
title: "[go] httpx: json/v2 and problem+json"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-5
depends_on: [VYQ-18]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-32 — [go] httpx: json/v2 and problem+json

## Scope

`internal/common/httpx`: decode with `encoding/json/v2` and `RejectUnknownMembers(true)`; encode; `application/problem+json` (RFC 9457) from SlugError. No chi.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/common/httpx/` green
- [ ] JSON with an unknown field is 400 problem+json
- [ ] no import of `encoding/json` v1

## Evidence & Notes

_Pending._
