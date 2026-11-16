---
id: VYQ-54
type: task
title: "[go] Prefix, 8-character codes, and attribution event"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-51
depends_on: [VYQ-52]
blocked_reason: null
updated: 2026-11-15
---

# VYQ-54 — [go] Prefix, 8-character codes, and attribution event

## Scope

Domain only, under `internal/links/domain`. Read ADR-0012 and ADR-0013
before editing. No HTTP, no SQL, no third-party module.

- `RandomCodeLength` becomes 8. Validation still accepts an existing
  public code of 3–64 characters. Reserved code names stay rejected.
- `Prefix` validates 1–16 characters matching `[A-Za-z0-9_-]+` and
  rejects reserved names, including case variants. Invalid input returns
  a typed error. The raw input is not part of the error text.
- `DerivePrefix` implements the ADR-0012 table. A reserved or empty
  derivation returns `d`. An explicit override is not derived; it is
  validated by `Prefix`.
- An attribution event is a value, not a request log. It carries site
  id, public code, type (`share` or `click`), optional channel,
  optional origin host, optional referrer host, optional device class,
  and the time. It has no IP, raw User-Agent, token, or destination URL.
  Device class is only `mobile`, `desktop`, `tablet`, `bot`, or
  `unknown`. Host fields are ASCII or punycode, without a path or query.
- Repository interfaces gain what storage needs to insert an event and
  to read per-site counts. Interfaces stay in the domain. No driver import.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/domain/...` exits 0, including `DerivePrefix` cases `unisimon.edu.co` → `uns`, `mi-tienda-online.com` → `mto`, `nike.com` → `nike`, `amazon.com` → `amz`, and `www.bbva.es` → `bbva`
- [ ] `go test ./internal/links/domain/link/ -run 'Code|Prefix'` shows `RandomCodeLength` 8 and rejects prefix `api`
- [ ] `go test ./internal/archtest/...` exits 0

## Evidence & Notes
