---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0005: Postgres and SQLite through `database/sql`

Status: Accepted

## Context and Problem Statement

Veylqo is self-hosted. A small install should run with one file. A
larger install should run on Postgres. The domain must not care which
one is configured, and the driver policy in
[ADR-0003](0003-stdlib-policy-and-driver-exceptions.md) already forbids
adapter packages from importing drivers.

How do we persist links on both engines without two business layers and
without a migration framework?

## Decision Drivers

* Adapters use `database/sql` only.
* Drivers are blank-imported only in `cmd/veylqo` (tests excepted, as in ADR-0003).
* SQLite is for small installs with one writer. It does not replace
  Postgres under load.
* No goose, no second migration tool in `go.mod`.
* The same repository suite must pass on memory, SQLite, and Postgres.
* Domain interfaces (`Repository`, `ClickRepository`) stay in `domain`.

## Considered Options

* One `database/sql` port, two adapters, embedded SQL migrations
* Postgres only
* SQLite only, with a later port
* An ORM or query generator shared by both engines

## Decision Outcome

Chosen option: "one `database/sql` port, two adapters, embedded SQL
migrations".

Adapters:

* Memory, for tests and as the shared-suite reference (VYQ-24).
* SQLite via `modernc.org/sqlite` (VYQ-26). Timestamps are stored as
  integer milliseconds. This is the small-install engine.
* Postgres via `github.com/jackc/pgx/v5/stdlib` (VYQ-27). The internal
  key uses the Postgres `uuid` type. `code` has a unique index. Links
  without a custom slug have a partial unique index on
  `destination_hash` so idempotent shorten can reuse a code.

Neither adapter imports its driver. See ADR-0003. SQL dialect differences
stay inside the adapter and the migration directory. They do not leak
into `app` or `ports`.

Migrations:

* SQL files embedded with `embed.FS`.
* Directories `migrations/postgres/` and `migrations/sqlite/`. Dialects
  are not forced into one file.
* A `schema_migrations` table records applied versions.
* One transaction per migration.
* Postgres takes an advisory lock so two processes do not migrate at once.
* No goose and no other migration library.

`cmd/veylqo` runs migrations on start. A flag may disable that for
operators who migrate out of band (VYQ-38). The migrator is application
infrastructure, not a domain service.

SQLite limits, which operators must be told and which this project will
not paper over:

* One writer. Do not claim SQLite matches Postgres under concurrent writes.
* A click flush and a shorten can contend on that writer. The batcher
  ([ADR-0009](0009-async-clicks-and-privacy.md)) keeps redirect latency
  off that lock, but it does not make SQLite a multi-writer database.

Postgres tests use `VEYLQO_TEST_PG_DSN` or `t.Skip`. A missing Postgres
in a laptop test run is not a product failure. CI that claims the
Postgres suite ran must actually provide the DSN (VYQ-41).

### Consequences

* Good, because domain and HTTP tests can use the memory repository.
* Good, because both engines implement the same interfaces and the same
  suite, so behavior drifts show up as test failures.
* Bad, because some SQL is duplicated across `migrations/postgres` and
  `migrations/sqlite`.
* Bad, because SQLite integer timestamps and Postgres `uuid` are easy to
  "simplify" into one dialect. Do not. Hide the difference in the adapter.
* Neutral, because choosing the engine is deployment configuration, not
  a domain concept. There is no runtime switch inside a use case.

### Confirmation

* VYQ-25: migrator tests apply embedded SQL to a temporary SQLite
  database and are idempotent. `go.mod` has no goose.
* VYQ-26: SQLite suite green; non-test adapter code does not import
  `modernc.org/sqlite`.
* VYQ-27: Postgres suite green or a documented skip; non-test adapter
  code does not import `github.com/jackc/pgx`.
* VYQ-38: production driver imports exist only in `cmd/veylqo`.

## Pros and Cons of the Options

### `database/sql`, two adapters, embedded migrations

* Good, because it fits ADR-0003 and the board's shared suite.
* Good, because a small install needs no separate database process.
* Bad, because we maintain two migration trees and a small migrator.

### Postgres only

* Good, because one dialect is less code.
* Bad, because the smallest self-hosted install would require Postgres.
  The board explicitly includes SQLite.

### SQLite only

* Good, because distribution is a single file.
* Bad, because one writer will not serve the load Postgres is there for,
  and we would pretend otherwise.

### ORM or query generator

* Good, because both engines could share a model definition.
* Bad, because that is a new `require` and usually codegen, both rejected.

## More Information

* Driver module names and import sites:
  [ADR-0003](0003-stdlib-policy-and-driver-exceptions.md).
* Idempotent shorten depends on the partial unique index above and on
  [ADR-0008](0008-url-cleaning.md) running before the lookup, so two
  equivalent URLs hash the same.
* Analytics writes go through the click repository and may be dropped:
  [ADR-0009](0009-async-clicks-and-privacy.md).
