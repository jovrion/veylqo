---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0003: Standard library first, with two driver exceptions

Status: Accepted

## Context and Problem Statement

The service must stay reviewable by agents and buildable without a
framework. Go 1.27 is new enough to provide `encoding/json/v2`,
`log/slog`, `net/http.ServeMux` pattern routing, and the standard `uuid`
package. Database access still needs a driver, and drivers are not in
the standard library.

Which dependencies are allowed in `go.mod`, and where may they be imported?

## Decision Drivers

* Go 1.27. No toolchain pin that pulls a framework.
* JSON requests use `encoding/json/v2` with `RejectUnknownMembers`.
* Identifiers use `uuid.NewV7()` from the standard `uuid` package.
* HTTP uses `net/http.ServeMux` only. Logging uses `log/slog`.
* Storage uses `database/sql` in adapters.
* No chi, codegen, YAML library, goose, testify, or third-party router.
* No new `require` without an ADR. Do not use the `tool` directive.
* Dev tools stay out of `go.mod`.
* `AGENTS.md` names this ADR as the authorization for the two drivers.

## Considered Options

* Standard library only, plus two drivers blank-imported in `cmd/veylqo`
* Standard library only, and refuse Postgres and SQLite drivers
* A small framework set (chi, goose, testify) plus drivers
* `database/sql` drivers imported by the adapter packages

## Decision Outcome

Chosen option: "standard library only, plus two drivers blank-imported
in `cmd/veylqo`".

Allowed in production code, with no extra `require`:

* `encoding/json/v2` for request and response bodies. Requests set
  `RejectUnknownMembers` so unknown fields are rejected. Do not import
  `encoding/json` v1 in production code.
* `uuid` (standard library) for `uuid.NewV7()`. No third-party UUID module.
* `net/http` and `net/http.ServeMux`. No chi, gin, echo, or other router.
* `log/slog`.
* `database/sql` in adapters. Adapters do not import a driver.
* `embed` for SQL migrations. No goose.
* `crypto/rand` for public codes. No third-party random or shortid library.
* Tests use the standard `testing` package, next to the code. No testify.

This ADR authorizes exactly two future `require` entries, when the
adapter tasks add them, and no others:

| Module | Role |
| --- | --- |
| `github.com/jackc/pgx/v5/stdlib` | Postgres driver for `database/sql` |
| `modernc.org/sqlite` | SQLite driver without cgo |

Import rules for those modules:

* Production blank imports live only in `cmd/veylqo`.
* Adapter packages, including non-test files under `internal/links/adapters`,
  must not import either driver.
* An adapter test may register a driver in `*_test.go` or a test helper
  so `go test` can run without the process main package.
* No other package may import them. `internal/archtest` enforces this
  once it exists (VYQ-12).

Anything else that wants a `require` needs a new ADR first. That
includes migration tools, routers, assertion libraries, OpenAPI
generators, and YAML parsers. Configuration is environment variables and
`flag`, not a YAML file.

The `tool` directive is not used. Linters, generators, and other dev
binaries are not module dependencies of the service.

`go.mod` does not exist yet (VYQ-11). This ADR does not add it, and it
does not add the driver lines early. VYQ-11 explicitly leaves drivers
out until the adapter tasks.

### Consequences

* Good, because the dependency list stays explainable: two drivers, each
  named here.
* Good, because `CGO_ENABLED=0` remains possible (`modernc.org/sqlite`,
  pgx stdlib).
* Bad, because we write HTTP helpers, problem+json encoding, and a small
  migrator ourselves.
* Bad, because a useful library still needs an ADR even when it is popular.
* Neutral, because tests may import a driver without making adapters
  depend on it. `archtest` must ignore `_test.go` for that rule or the
  suite cannot run.

### Confirmation

* VYQ-11: `go 1.27` in `go.mod`, and no driver `require` yet.
* VYQ-12: `go test ./internal/archtest/...` fails on `encoding/json` v1,
  a third-party router, a driver import outside `cmd/`, or a third-party
  import from `domain`.
* VYQ-26 and VYQ-27: non-test adapter packages do not import
  `modernc.org/sqlite` or `github.com/jackc/pgx`.
* VYQ-32: unknown JSON fields return 400 `application/problem+json`,
  and `httpx` does not import `encoding/json` v1.
* VYQ-38: only `cmd/veylqo` imports the drivers in production code.

## Pros and Cons of the Options

### Standard library plus two drivers in `cmd/veylqo`

* Good, because it matches `AGENTS.md` and still reaches both databases.
* Good, because driver registration cannot leak into the domain.
* Bad, because the first database task must touch `cmd/veylqo` as well
  as the adapter.

### No drivers at all

* Good, because `go.mod` would have zero third-party modules.
* Bad, because there is no standard-library Postgres or SQLite driver.
  A memory-only service is not the self-hosted product.

### Framework set plus drivers

* Good, because chi, goose, and testify shorten early tasks.
* Bad, because each one is a permanent API. The board already forbids them.

### Drivers imported by adapter packages

* Good, because tests and production share one import path.
* Bad, because any package that imports an adapter would pull a driver,
  and `archtest` could no longer keep drivers in `cmd/veylqo`.

## More Information

* Storage shape that these drivers serve:
  [ADR-0005](0005-postgres-and-sqlite-via-database-sql.md).
* Public identifiers still use the standard `uuid` package, not a driver
  type, as the application key:
  [ADR-0006](0006-internal-uuidv7-and-public-base62.md).
* This is the ADR `AGENTS.md` refers to as ADR-0003.
