---
id: VYQ-56
type: task
title: "[go] Prefix redirect, share events, and stats handlers"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-51
depends_on: [VYQ-53, VYQ-55]
blocked_reason: null
updated: 2026-11-15
---

# VYQ-56 — [go] Prefix redirect, share events, and stats handlers

## Scope

Application commands and queries, then `net/http.ServeMux` handlers.
Read ADR-0012 and ADR-0013. Spec first: the routes already documented
by VYQ-53. Ports contain no allowlist or cleaning rules. No new
`require`. Redirects stay 302, never 301.
`Cache-Control: private, max-age=0`.

- Shorten cleans, then checks the site allowlist, then allocates an
  8-character code unless the caller set a slug. Prefix comes from the
  request when present and valid, otherwise from `DerivePrefix`. The
  `201` body is `code`, `prefix`, and `short_url` built from
  `VEYLQO_PUBLIC_BASE_URL`. Creating a link records one `share` and does
  not wait on the batcher for the HTTP status.
- `GET /{prefix}/{code}` resolves only when the prefix matches. Mismatch
  and missing are 404. Expired or deleted is 410 and is not redirected.
  An active link is 302. `RecordClick` is not awaited. The click carries
  referrer host and device class only. `GET /{code}` still redirects an
  active link and still records a click.
- `POST /api/v1/links/{code}/shares` requires the owning site's
  publishable key. Another site's key is 404, not 403, so the response
  does not confirm the code. A cached copy in the SDK uses this route.
- `POST /api/v1/sites` and `GET /api/v1/sites` require the admin token.
  The create response includes the raw publishable key once. The list
  does not. The default env key still authorizes `POST /api/v1/links`
  for the default site.
- Stats stay admin-only and return the breakdowns from the spec. Do not
  log destination queries, IPs, or tokens.
- `internal/links/ports/contract_test.go` must accept the new spec paths
  and must fail if one side drops `GET /{prefix}/{code}`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/ports/... -run TestContract` exits 0
- [ ] an `httptest` of `GET /{prefix}/{code}` receives 302 and `Cache-Control: private, max-age=0`; a prefix mismatch is 404; an expired link is 410; no test expects 301
- [ ] `gofmt -s -w . && go vet ./... && go test ./... && go test ./internal/archtest/...` exits 0

## Evidence & Notes
