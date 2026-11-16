---
id: VYQ-53
type: task
title: "[contract] OpenAPI prefix links, shares, and attribution stats"
status: "open"
priority: "critical"
area: "contract"
epic: VYQ-51
depends_on: [VYQ-52]
blocked_reason: null
updated: 2026-11-15
---

# VYQ-53 — [contract] OpenAPI prefix links, shares, and attribution stats

## Scope

Edit `api/openapi/veylqo.json` only. Spec first (ADR-0004, ADR-0012,
ADR-0013). No handler yet. No YAML. No codegen. No new `require`.

Document, and do not invent fields beyond ADR-0012 and ADR-0013:

- `POST /api/v1/links` accepts optional `prefix`. `201` returns `code`,
  `prefix`, and `short_url`. `short_url` is an absolute http(s) URL with
  no query. The body must not echo the publishable key.
- `POST /api/v1/sites` (admin token) creates a site from `name`,
  `allowed_hosts`, and optional `prefix`. `201` returns `site_id`,
  `publishable_key` (once), and `prefix`. `GET /api/v1/sites` (admin
  token) lists sites and does not return keys.
- `POST /api/v1/links/{code}/shares` (publishable key of the owning site)
  records another copy. `204` or `202` with no destination body. A code
  owned by another site is `404`.
- `GET /{prefix}/{code}` is `302`, never `301`, with
  `Cache-Control: private, max-age=0`. Missing and prefix-mismatch are
  `404`. Expired or deleted is `410`.
- `GET /{code}` stays `302` and is unchanged in status rules.
- `GET /api/v1/links/{code}/stats` stays admin-only and adds `shares`,
  `clicks`, and breakdowns (`by_referrer`, `by_device`). Counts only.
  No IP, user agent, token, or destination.

Error responses stay `application/problem+json`. Do not add `301`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `python3 -c 'import json; json.load(open("api/openapi/veylqo.json"))'` exits 0
- [ ] the spec declares `302` for `GET /{prefix}/{code}` and `GET /{code}`, and no operation declares `301`
- [ ] `CreateLinkResponse` requires `code`, `prefix`, and `short_url`

## Evidence & Notes
