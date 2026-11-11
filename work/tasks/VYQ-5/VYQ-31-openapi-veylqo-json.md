---
id: VYQ-31
type: task
title: "[contract] OpenAPI 3.1 api/openapi/veylqo.json"
status: "done"
priority: "critical"
area: "contract"
epic: VYQ-5
depends_on: [VYQ-9]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-31 — [contract] OpenAPI 3.1 api/openapi/veylqo.json

## Scope

JSON spec (not YAML): `POST /api/v1/links`, `GET/DELETE /api/v1/links/{code}`, `GET /api/v1/links/{code}/stats`, `GET /healthz`, `GET /readyz`. problem+json errors. `GET /{code}` documented as a 302 redirect. No codegen.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `python3 -c 'import json; json.load(open("api/openapi/veylqo.json"))'`
- [x] the spec declares 302 for `GET /{code}` and 410 for expired links

## Evidence & Notes

- `python3 -c 'import json; json.load(open("api/openapi/veylqo.json"))'` exited 0.
- A follow-up load of `api/openapi/veylqo.json` asserted `paths["/{code}"].get.responses` contains `302` and `410`, the 410 description contains `expired`, `Cache-Control` on 302 is `private, max-age=0`, and no operation declares response `301`. Public `code` examples are `Ab3xyZ9`, not a UUID.
- Spec is hand-written OpenAPI 3.1 JSON. No YAML, no codegen, no new `require`. `/metrics` is not in this document; VYQ-38 adds that route later.
- `gofmt -s -w .`, `go vet ./...`, `go test ./...`, and `go test ./internal/archtest/...` — ok. No Go files changed.
