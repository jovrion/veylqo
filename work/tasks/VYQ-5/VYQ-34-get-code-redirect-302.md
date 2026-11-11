---
id: VYQ-34
type: task
title: "[go] GET /{code} 302 redirect"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-5
depends_on: [VYQ-21, VYQ-32]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-34 — [go] GET /{code} 302 redirect

## Scope

Outside `/api`. 302, `Cache-Control: private, max-age=0`, fires `RecordClick` without blocking. Reserved codes do not land here (`api`, `healthz`, `readyz`, `metrics`). Never 301.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] an httptest receives 302 and Location of the destination
- [x] an expired code is 410, a missing one 404
- [x] no test expects 301

## Evidence & Notes

- `gofmt -s -w .`, `go vet ./...`, `go test ./...`, and `go test ./internal/archtest/...` passed.
- `go test ./internal/links/ports/ -count=1 -race -timeout 60s` passed.
- 302 and Location: `TestRedirectReturns302AndLocation` serves `GET /Ab3xyZ9?token=secret` through `ports.NewRedirectMux` and the real Resolve query. Status is `http.StatusFound` (302), `Location` is the stored destination, and `Cache-Control` is `private, max-age=0`. The body is empty, so the destination query is not copied into it. `RecordClick` is fired and is not awaited (`TestRedirectDoesNotBlockOnRecordClick` returns 302 while `RecordClick.Handle` is still blocked). A click error does not change the status. The handler does not fetch the destination.
- Expired 410 and missing 404: `TestRedirectExpiredIs410AndMissingIs404` stores an expired link and a deleted link, then an httptest maps expired and deleted to 410 `gone` and a missing code to 404 `not-found`, both `application/problem+json` with an empty `Location`. A UUID is 404 and is not redirected. Those responses do not call `RecordClick`. The problem body does not contain the stored URL or `token=secret`.
- No test expects 301: success assertions require 302. `StatusMovedPermanently`, 307, and 308 are failure checks in `internal/links/ports/redirect_test.go` and `http_test.go`. The handler writes `http.StatusFound` and does not call `http.Redirect`. Reserved routes (`/api`, `/healthz`, `/readyz`, `/metrics`) are registered on `NewRedirectMux` and do not fall through to `GET /{code}` (`TestReservedRoutesDoNotRedirect`). A parent `GET /healthz` still wins when the redirect mux is mounted at `/`.
- `GET /{code}` is not on the API mux (`TestRedirectRouteIsNotRegistered`). Expiry stays in Resolve. The port only maps the slug.
