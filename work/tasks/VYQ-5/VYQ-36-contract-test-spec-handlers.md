---
id: VYQ-36
type: task
title: "[go] Contract test spec ↔ handlers"
status: "done"
priority: "major"
area: "go"
epic: VYQ-5
depends_on: [VYQ-33, VYQ-34]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-36 — [go] Contract test spec ↔ handlers

## Scope

Read `veylqo.json` with `encoding/json/v2` and assert every spec path and response code has a handler, and that there are no extra undocumented routes.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test` of the contract is green
- [x] removing a path from the spec or the mux fails the test

## Evidence & Notes

- `gofmt -s -w .`, `go vet ./...`, `go test ./...`, and `go test ./internal/archtest/...` passed.
- `go test ./internal/links/ports/ -count=1 -timeout 60s -run 'Contract|RemovingPath' -v` passed `TestContractSpecMatchesHandlers` and `TestRemovingPathFromSpecOrMuxFails`.
- The contract test reads `api/openapi/veylqo.json` with `encoding/json/v2`. Every spec method and path is registered on `NewMux` or `NewRedirectMux`, and `mux.Handler` reports that pattern. `GET /api` and `GET /metrics` are the only undocumented routes: they exist so those names do not fall through to `GET /{code}` (ADR-0007). `/metrics` stays out of the spec until VYQ-38.
- Response codes on implemented operations must be a handler success status (`201`, `200`, `204`, or `302`) or an edge status (`400`, `401`, `403`, `404`, `409`, `410`, `429`, `500`). `GET /{code}` documents `302` and the handler writes `http.StatusFound`. A documented `301` fails. `/healthz` and `/readyz` are registered; their `200` and `503` bodies stay VYQ-38.
- Removing a path fails inside the test: `alignRoutes` rejects a spec copy without `GET /healthz` and a mux copy without `POST /api/v1/links`. The live comparison uses the real spec and the slices `NewMux` and `NewRedirectMux` register from, so deleting a path from either side fails `TestContractSpecMatchesHandlers`.
