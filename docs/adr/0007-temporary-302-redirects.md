---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0007: Public redirects are 302, never 301

Status: Accepted

## Context and Problem Statement

`GET /{code}` sends the browser to the destination. A permanent redirect
is cached by browsers and intermediaries. Destinations change when a
link expires, is deleted, or is edited, and click counts only exist if
the request still reaches Veylqo.

Which redirect status do we send, and what must the handler not do?

## Decision Drivers

* `AGENTS.md`: redirects are **302**, never 301.
* `Cache-Control: private, max-age=0`.
* The redirect path is outside `/api` (VYQ-34).
* Expired is 410. Missing is 404. Neither is a redirect.
* Recording the click must not block the response
  ([ADR-0009](0009-async-clicks-and-privacy.md)).
* Reserved codes (`api`, `healthz`, `readyz`, `metrics`) are not handled
  by the redirect route.
* The OpenAPI document must say 302, not 301
  ([ADR-0004](0004-openapi-json-without-codegen.md)).

## Considered Options

* 302 with `Cache-Control: private, max-age=0`
* 301 permanent redirect
* 307 or 308 method-preserving redirect
* 200 with an HTML or JavaScript bounce

## Decision Outcome

Chosen option: "302 with `Cache-Control: private, max-age=0`".

`GET /{code}` behavior:

* Resolve the public code ([ADR-0006](0006-internal-uuidv7-and-public-base62.md)).
  Do not accept a UUID in this path.
* If the link is missing, respond 404 `application/problem+json`.
* If the link is expired or deleted, respond 410 `application/problem+json`.
  Do not redirect to the old destination.
* If the link is active, respond **302** with `Location` set to the
  stored destination and `Cache-Control: private, max-age=0`.
* Never respond 301. Tests must not expect 301. A 301 in a handler is a
  defect, not a cache optimization.
* Fire `RecordClick` without waiting for the click store. The redirect
  status does not depend on whether the click was queued.
* Do not log the destination URL with its query string, the client IP,
  or any token.

Reserved names are registered as their own routes (`/api/…`, `/healthz`,
`/readyz`, `/metrics`) so they never fall through to `GET /{code}`.
`debug` and `static` are reserved in the domain so they cannot be
created as codes even if no route uses them yet.

This status is also what the spec documents. Implementing 301 while the
spec says 302, or the reverse, fails the contract test.

### Consequences

* Good, because expiry and deletion take effect on the next request
  instead of after a browser cache expires.
* Good, because each visit can still be counted, within the limits of
  [ADR-0009](0009-async-clicks-and-privacy.md).
* Bad, because every visit pays a round trip to Veylqo. That is the
  product, not an accident. Operators who want a permanent CDN redirect
  are outside this design.
* Bad, because some clients treat 302 and 303 differently for non-GET
  methods. This route is GET. We do not advertise 302 as a general
  method-preserving redirect.
* Neutral, because `private, max-age=0` does not forbid a browser from
  remembering history. It forbids treating the redirect as a fresh
  shared cache entry.

### Confirmation

* VYQ-34: an `httptest` receives 302 and the destination `Location`;
  expired is 410; missing is 404; no test expects 301.
* VYQ-31: the spec declares 302 for `GET /{code}` and 410 for expired links.
* VYQ-21: Resolve returns the destination, `gone`, or `not-found`, and
  does not record the click itself.

## Pros and Cons of the Options

### 302 and `Cache-Control: private, max-age=0`

* Good, because it is the status `AGENTS.md` requires and caches poorly on purpose.
* Good, because intermediaries have no license to pin the destination.
* Bad, because click latency includes the origin. Async recording keeps
  that latency to the resolve path only.

### 301

* Good, because repeat clicks would be cheaper.
* Bad, because a deleted or expired link would keep redirecting from
  cache, and click counts would silently stop. Rejected.

### 307 or 308

* Good, because method and body are preserved.
* Bad, because the public action is a browser navigation via GET. 308
  is cached like 301. 307 is not what the spec and `AGENTS.md` require.
  Adding it later needs a new ADR.

### HTML or JavaScript bounce

* Good, because a page could show an interstitial.
* Bad, because it breaks non-browser clients, complicates caching, and
  is not a redirect status the contract can state as 302.

## More Information

* Destinations that appear in `Location` have already passed cleaning
  and allowlist checks at shorten time:
  [ADR-0008](0008-url-cleaning.md),
  [ADR-0010](0010-abuse-prevention.md).
* The resolve cache (VYQ-29) may cache the read model. It must not emit
  a 301, and it must drop entries on delete. Cache hits still use 302.
