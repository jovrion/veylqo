---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0008: Clean tracking parameters before storing a destination

Status: Accepted

## Context and Problem Statement

Shared links accumulate tracking queries (`utm_source`, `fbclid`, and
similar). Two users sharing the same page often produce different
strings, so idempotent shorten never matches, and stored URLs keep
third-party identifiers. Some sites also need a path shape preserved
(an Amazon `/dp/ASIN` is the product; a tracking suffix is not).

What do we strip, when can the caller opt out, and what is cleaning not
allowed to authorize?

## Decision Drivers

* VYQ-16: strip `utm_*`, `fbclid`, `gclid`, `ref`, `tag`, and similar;
  optional per-host rules; a `clean: false` option; table tests.
* VYQ-20: clean first, then enforce the host allowlist; reuse a code
  only when the cleaned destination matches.
* VYQ-15: destination is `http` or `https` only; reject `javascript:`
  and `data:`; drop the fragment; lowercase the host; block private and
  loopback hosts when that check is on.
* The standard library does not normalize IDN. Allowed hosts are ASCII
  or punycode (`AGENTS.md`).
* Do not log destination URLs with query strings.
* Cleaning is a domain service. It does not import HTTP or SQL.

## Considered Options

* Domain cleaner with a default tracking-parameter list, per-host rules,
  and `clean: false`
* Store the URL exactly as submitted
* Always clean, with no opt-out
* A third-party URL-normalization library

## Decision Outcome

Chosen option: "domain cleaner with a default tracking-parameter list,
per-host rules, and `clean: false`".

Default cleaning removes these query parameters (name match, case-insensitive):

* any parameter whose name starts with `utm_`
* `fbclid`
* `gclid`
* `ref`
* `tag`

"And similar" means other well-known click identifiers of the same kind
(`mc_eid`, `igshid`, `msclkid`, and the same class) may be added to that
default list in the domain cleaner as they are identified. Adding one is
an implementation detail of VYQ-16 if it stays inside this class. A new
class of rewriting (path changes for every host, body changes, or
fetching the URL) needs a new ADR.

Rules:

* Cleaning does not fetch the URL and does not follow redirects.
* The fragment is not stored. Destination normalization drops it
  (VYQ-15). Cleaning must not copy it back.
* The host is lowercased. The scheme stays `http` or `https`.
* A per-host rule may keep or reshape a path that would otherwise be
  ambiguous, for example preserving `/dp/ASIN` on a host that uses that
  shape. Rules are data in the domain service, not HTTP middleware.
* `clean: false` skips parameter stripping and per-host path rules. It
  does **not** skip scheme checks, private-host checks, fragment
  dropping, or the allowlist. Opting out of cleaning is not opting out
  of safety ([ADR-0010](0010-abuse-prevention.md)).
* Idempotent reuse hashes the post-clean destination. Two dirty URLs
  that clean to the same destination reuse one code when the existing
  link has no custom slug and no expiry (VYQ-20).
* Empty query strings are omitted. Parameter order of the remaining
  query should be stable so equal destinations compare equal.

Internationalized names:

* Do not call a non-existent standard-library IDN normalizer.
* Compare and store hosts as ASCII or punycode. A Unicode host that the
  caller has not converted to punycode is rejected rather than silently
  rewritten by an outside library.

Logging: do not log the raw or cleaned URL with its query string. A log
line may include the public code and an error slug.

This ADR does not add a dependency. The cleaner uses `net/url`.

### Consequences

* Good, because the same article shared from two campaigns becomes one
  short link, unless the caller set a custom slug, an expiry, or
  `clean: false`.
* Good, because stored destinations drop common third-party click ids.
* Bad, because a caller who needed `ref` or `tag` as application data
  must pass `clean: false`, and must accept that those values are then
  stored.
* Bad, because "similar" can grow the default list and change a
  previously stored equality. New names apply to new shortens. They do
  not rewrite existing rows.
* Neutral, because per-host path rules are optional. The default
  installation still strips the query list above and does not need a
  host table to be correct.

### Confirmation

* VYQ-16: `go test ./internal/links/domain/link/ -run Clean`. A URL with
  `utm_source` comes out without that parameter.
* VYQ-15: `javascript:alert(1)` and, when blocking is on,
  `http://127.0.0.1/` fail destination validation. Cleaning cannot make
  those succeed.
* VYQ-20: a host outside the allowlist is `incorrect-input` even if
  cleaning succeeded.

## Pros and Cons of the Options

### Domain cleaner, default list, per-host rules, `clean: false`

* Good, because policy lives next to the destination value object.
* Good, because callers with a legitimate tracking parameter can opt out.
* Bad, because the default list is a judgment call and will need occasional additions.

### Store the URL exactly as submitted

* Good, because there is no surprise rewriting.
* Bad, because idempotency fails on ordinary campaign links, and we store
  identifiers the sharer did not mean to keep. Rejected.

### Always clean, no opt-out

* Good, because one code path.
* Bad, because some destinations use `ref` or `tag` as real parameters.
  The board requires `clean: false`.

### Third-party normalizer

* Good, because lists of tracking parameters already exist in libraries.
* Bad, because that is a new `require` without a need the standard
  `net/url` cannot meet. Rejected by
  [ADR-0003](0003-stdlib-policy-and-driver-exceptions.md).

## More Information

* This is the ADR VYQ-16 names.
* Allowlists and private-host blocking:
  [ADR-0010](0010-abuse-prevention.md).
* Equality after cleaning is what the partial unique index in
  [ADR-0005](0005-postgres-and-sqlite-via-database-sql.md) enforces.
