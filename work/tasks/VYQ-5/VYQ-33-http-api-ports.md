---
id: VYQ-33
type: task
title: "[go] HTTP API ports"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-5
depends_on: [VYQ-23, VYQ-31, VYQ-32]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-33 — [go] HTTP API ports

## Scope

`internal/links/ports/http.go` implements the spec by hand on `ServeMux`. Maps slugs to 400/401/404/409/410/429. No business logic: only calls Commands/Queries.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/ports/` covers the spec HTTP codes
- [x] `ports` does not import `database/sql`

## Evidence & Notes

- `gofmt -s -w .`, `go vet ./...`, `go test ./...`, and `go test ./internal/archtest/...` — ok. `go test ./internal/links/ports/` — ok.
- Spec codes: `TestSpecHTTPCodes` maps incorrect-input to 400, unauthorized to 401, not-found to 404, conflict to 409, gone to 410, and rate-limited to 429, each as `application/problem+json`. Unknown JSON fields are 400 and do not call Shorten. A non-slug error is 500 with a fixed detail. Success paths are 201, 200, and 204. `GET /{code}` is not registered.
- No SQL import: `TestPortsDoesNotImportDatabaseSQL` parses production imports in `internal/links/ports` and rejects `database/sql`. Handlers only call Commands and Queries. Redirects, keys, and the limiter are not this package.
