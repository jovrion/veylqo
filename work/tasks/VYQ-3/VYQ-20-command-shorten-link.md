---
id: VYQ-20
type: task
title: "[go] command ShortenLink"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-3
depends_on: [VYQ-16, VYQ-17, VYQ-18]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-20 — [go] command ShortenLink

## Scope

Clean the URL, check the host allowlist, reuse the code if the URL already exists and the link has neither a custom slug nor expiry, otherwise generate a code and retry on collision. Idempotent.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/app/command/ -run Shorten` green
- [ ] two shortenings of the same URL without a slug return the same code
- [ ] a host outside the allowlist is `incorrect-input`

## Evidence & Notes

_Pending._
