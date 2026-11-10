---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0002: Clean Architecture and the Wild Workouts patterns that apply

Status: Accepted

## Context and Problem Statement

Veylqo is a small link service, but it still has a domain rule set
(codes, destinations, expiry, cleaning) that must not depend on HTTP or
SQL. The board already splits work into domain, application, adapters,
ports, and service composition, and it names the Wild Workouts style for
commands and queries.

Which of those patterns do we adopt, and which parts of that reference
do we refuse because they conflict with the standard-library policy?

## Decision Drivers

* `domain` must not import `app`, `adapters`, `ports`, `service`, or
  third-party modules.
* `app` knows domain interfaces only. No `net/http` and no SQL in `app/`.
* `ports` contains no business rules.
* Composition lives only in `internal/links/service` and `cmd/veylqo`.
* Commands mutate. Queries only read.
* Decorators contain no business logic.
* Expected failures are returned errors, never `panic`.
* No chi, codegen, YAML, goose, testify, or third-party router.

## Considered Options

* Layered `internal/links` plus the Wild Workouts command/query shape
* A single `internal/links` package with handlers calling SQL
* Full Wild Workouts stack, including Watermill, chi, and their HTTP helpers
* Hexagonal ports in a separate module per layer

## Decision Outcome

Chosen option: "layered `internal/links` plus the Wild Workouts
command/query shape", because it matches the board and keeps
infrastructure out of the domain without taking that project's libraries.

Layout under `internal/links`:

| Layer | Role |
| --- | --- |
| `domain` | Entities and value objects. Repository interfaces live here as domain interfaces. |
| `app` | Commands and queries. Depends on domain interfaces, not on adapters. |
| `adapters` | SQL, batching, caches, code generation. Implements domain interfaces. |
| `ports` | HTTP (and similar) edge. Calls commands and queries. No business rules. |
| `service` | Wires the application. No use-case logic. |

`cmd/veylqo` is the process entrypoint: configuration, driver imports,
migrations, and server lifecycle. It is not a sixth business layer.

Domain rules:

* Private fields. `NewLink` validates. `UnmarshalLinkFromDatabase`
  rehydrates a stored row and does not re-run creation policy that would
  reject a historical row for a reason other than corruption.
* Typed domain errors. No `panic` for expected errors.
* `domain` does not import `net/http` or `database/sql`.

Application rules, in the Wild Workouts style that applies here:

* `app.Application` exposes `Commands` and `Queries`.
* Commands: Shorten, RecordClick, Delete. Queries: Resolve, Get, Stats.
  Resolve does not record a click.
* Handlers are wrapped in decorators before they leave the application
  constructor. Decorator order is recovery, then logging, then metrics.
* Decorators may log a stable `Name()` and the error class. They must
  not log destination URLs, query strings, IPs, or tokens, and they must
  not decide allowlists, expiry, or cleaning.
* Recovery turns a handler panic into an error so one request cannot
  kill the process.
* Application errors use stable slugs (`SlugError`) that ports map to
  HTTP status codes. The slug is not an HTTP concern inside `app`.

What we do not take from Wild Workouts: chi, Watermill, a message bus,
YAML service config, goose, testify, protobuf, or generated OpenAPI
servers. Those are rejected by [ADR-0003](0003-stdlib-policy-and-driver-exceptions.md)
and [ADR-0004](0004-openapi-json-without-codegen.md).

### Consequences

* Good, because a domain test needs no database and no HTTP server.
* Good, because swapping SQLite and Postgres cannot change cleaning,
  expiry, or code rules.
* Bad, because a small feature touches more packages than a single handler file.
* Bad, because decorator order is easy to invert. The application
  constructor is the only place that wraps handlers.

### Confirmation

Planned, not done, until the board says so:

* VYQ-12: `internal/archtest` fails if `domain` imports `app`,
  `adapters`, `ports`, `service`, or a third-party module.
* VYQ-17: `NewLink` rejects an invalid code or destination; `domain`
  does not import `net/http` or `database/sql`.
* VYQ-19: a panic in a handler becomes an error.
* VYQ-23: handlers leave `app` already wrapped. No SQL or HTTP wiring there.

## Pros and Cons of the Options

### Layered `internal/links` plus Wild Workouts commands and queries

* Good, because the board, `AGENTS.md`, and the reference implementation
  agree on commands, queries, decorators, and an `Application` struct.
* Good, because repository interfaces stay in the domain, so `app` does
  not import adapters.
* Bad, because contributors who know only `net/http` handlers will see
  more files.

### Single package, handlers call SQL

* Good, because the first endpoint is faster to write.
* Bad, because domain rules become untestable without a database, and
  ports gain business rules. Rejected by `AGENTS.md`.

### Full Wild Workouts stack

* Good, because the tutorials already show HTTP and Postgres wiring.
* Bad, because chi, Watermill, and their helper libraries violate the
  standard-library policy.

### One module per layer

* Good, because import boundaries become Go module boundaries.
* Bad, because this is one binary and one bounded context. Modules would
  add replace directives and no extra safety beyond `archtest`.

## More Information

* Reference shape, not a dependency: Three Dots Labs, Wild Workouts
  (<https://github.com/ThreeDotsLabs/wild-workouts-go>).
* HTTP mapping and status codes: [ADR-0007](0007-temporary-302-redirects.md),
  ports tasks VYQ-33 and VYQ-34.
* Storage behind the domain interfaces: [ADR-0005](0005-postgres-and-sqlite-via-database-sql.md).
