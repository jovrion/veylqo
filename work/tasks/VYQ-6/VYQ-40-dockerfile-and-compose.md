---
id: VYQ-40
type: task
title: "[ops] Dockerfile and docker-compose"
status: "open"
priority: "major"
area: "ops"
epic: VYQ-6
depends_on: [VYQ-38]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-40 — [ops] Dockerfile and docker-compose

## Scope

Multi-stage Dockerfile, `CGO_ENABLED=0`, distroless. Compose: postgres + veylqo. Document `VEYLQO_ALLOWED_HOSTS`, `VEYLQO_BASE_URL`, keys.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `test -f Dockerfile -a -f docker-compose.yml`
- [ ] the Dockerfile sets `CGO_ENABLED=0`

## Evidence & Notes

_Pending._
