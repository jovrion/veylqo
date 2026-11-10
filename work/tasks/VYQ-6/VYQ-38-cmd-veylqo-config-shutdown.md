---
id: VYQ-38
type: task
title: "[go] cmd/veylqo: config, drivers, shutdown"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-6
depends_on: [VYQ-37, VYQ-33, VYQ-34, VYQ-35]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-38 — [go] cmd/veylqo: config, drivers, shutdown

## Scope

`cmd/veylqo/main.go`: env + `flag`, blank imports of pgx and sqlite, migrations on start (disableable), `signal.NotifyContext` + `Server.Shutdown` that drains the batcher. `/healthz`, `/readyz`, `/metrics`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go build -o /tmp/veylqo ./cmd/veylqo`
- [ ] only `cmd/veylqo` imports the drivers (archtest)
- [ ] a SIGTERM in test does not skip the batcher flush

## Evidence & Notes

_Pending._
