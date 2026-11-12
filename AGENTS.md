# Veylqo — Agent Instructions

Self-hosted, open-source link shortener and share service.
Go 1.27, standard library first. The SDK lives in `jovrion/veylqo-js`
and is tracked here as area `js`.

Tagline: Less URL. More destination.

## Source-of-truth map (read on demand)

- `WORKFLOW.md`: task cycle, board commands, slash commands.
- `work/README.md` + `work/BOARD.md`: tracking. IDs `VYQ-<n>`,
  statuses `open` / `in-progress` / `blocked` / `done`.
  Use `make work-next`, `make work-start ID=VYQ-<n>`, `make work-done ID=VYQ-<n>`.
- `docs/architecture.md`: layers, shorten/resolve flows, security.
  Created by VYQ-10. Do not invent a parallel design.
- `docs/adr/`: accepted decisions. Read the ADR named by the task
  before editing. An architecture change needs a new ADR.

## Agents and commands in this repository (`.opencode/`)

- `planner` (read-only): design, trade-offs, and verifiable plans.
- `builder-go`: Go backend (`cmd/`, `internal/`) and the OpenAPI contract.
- `builder-js`: SDK in `jovrion/veylqo-js` (vanilla Web Component).
- `architecture-guardian` (subagent, `/review`): layers, stdlib policy,
  abuse controls, redirect semantics.
- `debugger` (subagent, read-only): redirect, expiry, abuse, or database incidents.
- Board commands: `/work-next`, `/work-start VYQ-<n>`, `/work-done VYQ-<n>`
  (see `WORKFLOW.md`).

## Architecture rules (non-negotiable)

- Layers in `internal/links`: `domain` must not import `app`, `adapters`,
  `ports`, or third-party modules. `app` only knows domain interfaces.
  `ports` contains no business rules. Composition lives in
  `internal/links/service` and `cmd/veylqo`.
- Domain: private fields, `NewLink` validates, `UnmarshalLinkFromDatabase`
  rehydrates. No `panic` for expected errors.
- Commands mutate. Queries only read. Decorators (logging, metrics)
  contain no business logic. Order: recovery → logging → metrics.
- HTTP: `net/http.ServeMux` only. JSON: `encoding/json/v2` with
  `RejectUnknownMembers` on requests. Errors:
  `application/problem+json` (RFC 9457). Redirects: **302**, never 301.
  `Cache-Control: private, max-age=0`.
- Identifiers: internal key `uuid.NewV7()` from the standard `uuid`
  package. The public code is base62, distinct from the UUID.
- Storage: adapters use `database/sql` only. Drivers
  (`github.com/jackc/pgx/v5/stdlib`, `modernc.org/sqlite`) are blank-imported
  only in `cmd/veylqo`.
- Contract: `api/openapi/veylqo.json` (OpenAPI 3.1 JSON, no YAML).
  Spec first, then the handler, then the contract test must pass.
  No generated files to edit by hand.
- Dependencies: no new `require` in `go.mod` without an ADR. The only
  planned exceptions are the two drivers (ADR-0003, once it exists).
  Dev tools stay out of `go.mod`. Do not use the `tool` directive.
- Public shortening is not an open redirect: `VEYLQO_ALLOWED_HOSTS` is
  required, `http`/`https` only, block private and loopback hosts,
  per-IP rate limit, publishable key distinct from the admin token.
- Do not log destination URLs with query strings, IPs, or tokens.
  Click analytics are counts, not raw request logs.
- A task describes intent. Do not claim a feature exists unless its
  task is `done`.

## Verification and tracking

- Before implementing, find the task with `make work-next` and claim it
  with `make work-start ID=VYQ-<n>`.
- Close with `make work-done ID=VYQ-<n>` only when every
  `## Verification` checkbox has re-runnable evidence. Then
  `make work-validate`.
- Go, once code exists: `gofmt -s -w . && go vet ./... && go test ./...`.
  Once the package exists: `go test ./internal/archtest/...`.
- `js` tasks run in `jovrion/veylqo-js`
  (`git@github.com-unisimon:jovrion/veylqo-js.git`). Record evidence
  on the task in this repo.
- No commit or push unless the user asks.
  Remote: `git@github.com-unisimon:jovrion/veylqo.git`.
  `main` is protected. A requested push goes to a branch and a pull
  request, never a direct push to `main`.

## Limits

- SQLite is for small installs (one writer). Do not claim it replaces
  Postgres under load.
- Analytics are approximate: a click can be lost if the process dies
  before flush. Document that; do not promise exactly-once.
- The standard library does not normalize IDN. Allowed hosts are ASCII
  or punycode.
