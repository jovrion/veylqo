---
id: VYQ-32
type: task
title: "[go] httpx: json/v2 and problem+json"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-5
depends_on: [VYQ-18]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-32 — [go] httpx: json/v2 and problem+json

## Scope

`internal/common/httpx`: decode with `encoding/json/v2` and `RejectUnknownMembers(true)`; encode; `application/problem+json` (RFC 9457) from SlugError. No chi.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/common/httpx/` green
- [x] JSON with an unknown field is 400 problem+json
- [x] no import of `encoding/json` v1

## Evidence & Notes

- `gofmt -s -w .`, `go vet ./...`, `go test ./...`, and `go test ./internal/archtest/...` — ok. `go test ./internal/common/httpx/` — ok.
- Unknown field: `TestUnknownFieldIs400ProblemJSON`. `ReadJSON` of `{"url":"...","extra":true}` is 400 `application/problem+json`, slug `incorrect-input`, and the instance is the path without the query string. The body does not include the destination URL, the query, or the client IP.
- No v1 import: `TestPackageDoesNotImportJSONV1OrChi` parses production imports. `archtest` stdlib-only rule also rejects `encoding/json` v1 and chi.
- Decode uses `encoding/json/v2` and `RejectUnknownMembers(true)`. `WriteProblem` maps the six slugs to 400, 401, 404, 409, 410, and 429. A non-slug error is 500 with a fixed detail.
