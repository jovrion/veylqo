---
id: VYQ-38
type: task
title: "[go] cmd/veylqo: config, drivers, shutdown"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-6
depends_on: [VYQ-37, VYQ-33, VYQ-34, VYQ-35]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-38 — [go] cmd/veylqo: config, drivers, shutdown

## Scope

`cmd/veylqo/main.go`: env + `flag`, blank imports of pgx and sqlite, migrations on start (disableable), `signal.NotifyContext` + `Server.Shutdown` that drains the batcher. `/healthz`, `/readyz`, `/metrics`.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go build -o /tmp/veylqo ./cmd/veylqo`
- [x] only `cmd/veylqo` imports the drivers (archtest)
- [x] a SIGTERM in test does not skip the batcher flush

## Evidence & Notes

2026-11-11:

```
gofmt -s -w .
go vet ./...
go test ./... -count=1
ok  github.com/jovrion/veylqo/cmd/veylqo
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/links/ports
ok  github.com/jovrion/veylqo/internal/links/service
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
go build -o /tmp/veylqo ./cmd/veylqo
make work-validate
work: OK (48 items, 0 warnings)
```

`TestSIGTERMDoesNotSkipBatcherFlush` builds `./cmd/veylqo`, queues one click (batch 1000, interval 1h, count still 0), sends SIGTERM, and reads `clicks.click_count = 1` after exit. stderr has no destination query, publishable key, or admin token. `TestHealthAndMigrateFlag` checks `VEYLQO_MIGRATE=false` leaves `clicks` uncreated. Drivers are blank-imported only in `cmd/veylqo/main.go`; archtest rejects any other production import.
