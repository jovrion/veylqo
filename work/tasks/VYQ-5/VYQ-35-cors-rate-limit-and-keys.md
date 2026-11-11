---
id: VYQ-35
type: task
title: "[go] CORS, rate limit, and keys"
status: "done"
priority: "major"
area: "go"
epic: VYQ-5
depends_on: [VYQ-32]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-35 — [go] CORS, rate limit, and keys

## Scope

Publishable key for shortening; admin token for GET/DELETE/stats. CORS for configured origins. Per-IP token bucket (`net/netip`, `VEYLQO_TRUSTED_PROXIES`). `http.CrossOriginProtection` on admin.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test` of the limiter and auth middleware is green
- [x] without a publishable key, POST /api/v1/links is 401

## Evidence & Notes

- `gofmt -s -w .`, `go vet ./...`, `go test ./...`, and `go test ./internal/archtest/...` passed.
- `go test ./internal/links/ports/ -count=1 -race -timeout 90s` passed.
- Limiter: `TestLimiterTokenBucket` allows one request, then 429 `rate-limited` `application/problem+json` with `Cache-Control: private, max-age=0`. A second IP is a separate bucket. Advancing the injected clock by the refill interval allows the first IP again. The body does not contain `192.0.2.10` or the untrusted `X-Forwarded-For` value. The response is not 301 and has no `Location`.
- Trusted proxies: `ParseTrustedProxies` reads a comma-separated `VEYLQO_TRUSTED_PROXIES` value (IP or CIDR). An invalid entry fails without echoing it. `TestLimiterIgnoresUntrustedForwardedHeader` keeps the connection address when the peer is not trusted. `TestLimiterUsesForwardedHeaderOnlyForTrustedPeer` uses `X-Forwarded-For` only for a peer in `10.0.0.0/8`, skipping a trusted hop. An unparseable forwarded header is ignored (`TestLimiterIgnoresUnparseableForwardedHeader`) and does not leak `token=secret`. An unparseable peer fails closed with 429 (`TestLimiterFailClosedOnUnparseablePeer`). IPv6 peers share a bucket (`TestLimiterIPv6Peer`). `TestLimiterIsSafeForConcurrentClients` allows exactly the burst under `-race`.
- Auth: `TestPostWithoutPublishableKeyIs401` serves `POST /api/v1/links?token=secret` through `NewAPIGuard(NewMux(...))` with no `Authorization` header. Status is 401, slug `unauthorized`, `WWW-Authenticate: Bearer`, and the shorten command is not called. The body does not contain the query token, the destination, or the configured key. `TestWrongCredentialDoesNotCrossRoutes` rejects a wrong bearer, the admin token on shorten, a key in the query string, and the publishable key on GET, DELETE, and stats. `TestPublishableKeyReachesShorten` returns 201. `TestAdminTokenReachesGetDeleteAndStats` reaches GET, DELETE, and stats. `TestRedirectDoesNotRequirePublishableKey` leaves `GET /{code}` at 302 with no credential.
- CORS and admin CSRF: configured origins are reflected exactly, never `*`. A disallowed origin is not reflected and is not copied into the body. Allowed preflight is 204 and does not call the handler. `TestAdminCrossOriginProtection` wraps admin routes with `http.CrossOriginProtection`: a cross-site DELETE is 403 `application/problem+json` and does not include the origin; the same delete from a configured origin reaches the handler. Shorten is not behind that check, so a cross-site POST with the publishable key still returns 201. Equal keys, an empty key, `*`, and an origin with a query are rejected at construction without echoing the value.
- `NewMux` itself still does not check keys. `cmd` and `service` do not exist yet; they must wrap the API mux with `NewAPIGuard` and the redirect mux with `Limiter.Handler`. This package does not read environment variables.
