---
id: VYQ-31
type: task
title: "[contract] OpenAPI 3.1 api/openapi/veylqo.json"
status: "open"
priority: "critical"
area: "contract"
epic: VYQ-5
depends_on: [VYQ-9]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-31 — [contract] OpenAPI 3.1 api/openapi/veylqo.json

## Scope

JSON spec (not YAML): `POST /api/v1/links`, `GET/DELETE /api/v1/links/{code}`, `GET /api/v1/links/{code}/stats`, `GET /healthz`, `GET /readyz`. problem+json errors. `GET /{code}` documented as a 302 redirect. No codegen.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `python3 -c 'import json; json.load(open("api/openapi/veylqo.json"))'`
- [ ] the spec declares 302 for `GET /{code}` and 410 for expired links

## Evidence & Notes

_Pending._
