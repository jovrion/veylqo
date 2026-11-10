---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0006: Internal UUIDv7, public base62 code

Status: Accepted

## Context and Problem Statement

Every link needs a stable primary key and a short public identifier.
Using one string for both either leaks a time-ordered UUID in the URL or
makes the primary key a short guessable token. The board already splits
them: `uuid.NewV7()` internally, a base62 code in the path.

What is the public code, what is the internal key, and what strings must
never be used as a code?

## Decision Drivers

* Internal key is `uuid.NewV7()` from the standard `uuid` package
  ([ADR-0003](0003-stdlib-policy-and-driver-exceptions.md)).
* The public code is base62 and is not the UUID (`AGENTS.md`).
* Random codes are 7 characters. Custom slugs are 3–64 characters.
* Reserved names must not be capturable as links: `api`, `healthz`,
  `readyz`, `metrics`, `debug`, `static`.
* Generation uses `crypto/rand`, not `math/rand`.
* The code is the only identifier in `GET /{code}` and in the public API path.

## Considered Options

* UUIDv7 primary key plus a separate base62 public code
* UUIDv7 in the public path
* Base62 as the primary key, no UUID
* Sequential integers in the public path

## Decision Outcome

Chosen option: "UUIDv7 primary key plus a separate base62 public code".

Internal key:

* Created with `uuid.NewV7()` in the standard library `uuid` package.
* Stored as `uuid` on Postgres and as a 16-byte or canonical string form
  on SQLite, hidden inside the adapter
  ([ADR-0005](0005-postgres-and-sqlite-via-database-sql.md)).
* Never placed in a redirect path, a share URL, or a public JSON `code` field.
* Not parsed from client input. Clients address links by public code.

Public code:

* Random codes: length 7, base62 alphabet (`0-9`, `A-Z`, `a-z`).
* Generator uses `crypto/rand` and rejection sampling so the distribution
  is unbiased (VYQ-30). No `math/rand`, no shortid module.
* Custom slugs: length 3–64, matching `[a-zA-Z0-9_-]+`.
* Rejected, even if they match the regex: `api`, `healthz`, `readyz`,
  `metrics`, `debug`, `static`. Comparison is case-sensitive against
  those lowercase reserved words; a slug that differs only by case is
  still rejected if folding it equals a reserved word, so `API` cannot
  shadow `api`.
* The validating constructor lives in the domain (`Code`). Adapters do
  not invent a second alphabet.

Collision handling belongs to the shorten command: generate, insert,
retry on unique conflict. The generator itself does not consult the
database.

Idempotent reuse of an existing code for the same destination
(VYQ-20) applies only when the existing link has neither a custom slug
nor an expiry. That reuse returns the existing public code. It does not
return the UUID.

### Consequences

* Good, because logs and support can correlate a row by UUID without
  putting that UUID on the public internet.
* Good, because UUIDv7 indexes well as a primary key, while the public
  code stays short.
* Bad, because every link has two identifiers, and APIs must not mix them.
* Bad, because 7-character codes are guessable over a large space only
  in the practical sense: they are unguessable enough for an untrusted
  redirect if abuse controls hold, and they are not a capability token
  for admin operations. Admin actions use the admin token
  ([ADR-0010](0010-abuse-prevention.md)), not knowledge of the code.
* Neutral, because custom slugs are optional. Absence of a slug means
  "generate base62", not "use the UUID".

### Confirmation

* VYQ-14: `go test ./internal/links/domain/link/ -run Code`. Slugs `api`
  and `healthz` fail. Random shape is 7-character base62.
* VYQ-30: 1000 consecutive generator outputs have length 7 and stay in
  the base62 alphabet.
* A grep of the public JSON examples in the OpenAPI spec, once it exists,
  shows `code` as the short form, not a UUID. That check belongs to VYQ-31.

## Pros and Cons of the Options

### UUIDv7 plus base62 code

* Good, because it matches `AGENTS.md` and keeps the URL short.
* Good, because the primary key can be time-ordered without being the share token.
* Bad, because two identifiers must be mapped in every adapter.

### UUIDv7 in the path

* Good, because there is only one identifier.
* Bad, because the URL is long, the tagline is "Less URL", and a
  time-ordered id leaks creation order to anyone who sees a link.

### Base62 as the only key

* Good, because the schema has one string column.
* Bad, because random short keys cluster poorly as primary keys, and a
  custom slug would be the only key a deleted-and-recreated link could collide on.

### Sequential integers

* Good, because they are short.
* Bad, because they are enumerable. Neighbors can walk `/1`, `/2`, `/3`
  and harvest destinations. Rejected.

## More Information

* Redirect routes use the public code only:
  [ADR-0007](0007-temporary-302-redirects.md).
* Cleaning does not change the code. It changes the stored destination:
  [ADR-0008](0008-url-cleaning.md).
