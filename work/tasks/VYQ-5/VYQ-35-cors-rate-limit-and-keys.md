---
id: VYQ-35
type: task
title: "[go] CORS, rate limit, and keys"
status: "open"
priority: "major"
area: "go"
epic: VYQ-5
depends_on: [VYQ-32]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-35 — [go] CORS, rate limit, and keys

## Scope

Publishable key for shortening; admin token for GET/DELETE/stats. CORS for configured origins. Per-IP token bucket (`net/netip`, `VEYLQO_TRUSTED_PROXIES`). `http.CrossOriginProtection` on admin.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test` of the limiter and auth middleware is green
- [ ] without a publishable key, POST /api/v1/links is 401

## Evidence & Notes

_Pending._
