---
id: VYQ-41
type: task
title: "[ops] CI e2e against Postgres"
status: "done"
priority: "normal"
area: "ops"
epic: VYQ-6
depends_on: [VYQ-40, VYQ-13]
blocked_reason: null
updated: 2026-11-12
---

# VYQ-41 — [ops] CI e2e against Postgres

## Scope

CI job with a Postgres service and `VEYLQO_TEST_PG_DSN`. Postgres adapter tests stop skipping.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] the workflow has a job with Postgres
- [x] the job exports `VEYLQO_TEST_PG_DSN`

## Evidence & Notes

2026-11-12. Job `postgres` in `.github/workflows/ci.yml` runs `postgres:16-alpine` (same image as Compose) and sets `VEYLQO_TEST_PG_DSN` at job level, before `steps`. The existing `ci` job still runs without a database, so a missing Postgres on that path is not a failure (ADR-0005). The postgres job fails if the adapter tests skip. The DSN is not printed. No new `require`.

```
grep -n -E 'postgres:|image: postgres|VEYLQO_TEST_PG_DSN' .github/workflows/ci.yml
# 87:  postgres:
# 91:      postgres:
# 92:        image: postgres:16-alpine
# 107:      VEYLQO_TEST_PG_DSN: postgres://...@127.0.0.1:5432/veylqo?sslmode=disable
# 120:          if [ -z "${VEYLQO_TEST_PG_DSN:-}" ]; then
# 121:            echo "VEYLQO_TEST_PG_DSN is unset"

# same test command as the job, against a temporary postgres:16-alpine
# VEYLQO_TEST_PG_DSN set, not printed
go test ./internal/links/adapters/ -count=1 -run 'Postgres' -v
# --- PASS: TestPostgresRepository (0.22s)
# --- PASS: TestPostgresStoresIDAsUUID (0.04s)
# no --- SKIP
# postgres adapter tests: no skip

gofmt -s -w .
go vet ./...
go test ./...
go test ./internal/archtest/...
# ok
make work-validate
# work: OK (48 items, 0 warnings)
```
