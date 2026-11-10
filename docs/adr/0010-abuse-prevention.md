---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0010: Public shortening is not an open redirect

Status: Accepted

## Context and Problem Statement

A public shorten endpoint that accepts any URL is an open redirect and
an SSRF-shaped footgun: an attacker stores `http://127.0.0.1/` or a
private-network URL, then sends victims through the short link, or
induces the operator's host to trust it. The same endpoint, if
unauthenticated, will be used as a free redirector.

What must be true before a destination is stored, and which credentials
gate which routes?

## Decision Drivers

* `VEYLQO_ALLOWED_HOSTS` is required (`AGENTS.md`).
* `http` and `https` only. Block private and loopback hosts.
* Per-IP rate limit. Publishable key distinct from the admin token.
* Do not log destination URLs with query strings, IPs, or tokens.
* Allowed hosts are ASCII or punycode
  ([ADR-0008](0008-url-cleaning.md)).
* Ports map failures to 400, 401, 404, 409, 410, and 429 (VYQ-33).
* `POST /api/v1/links` without the publishable key is 401 (VYQ-35).

## Considered Options

* Required allowlist, scheme and address checks, two credentials, per-IP limit
* Authenticated shorten, but any public destination host
* Block private hosts only, allow any public host
* CAPTCHA or an external abuse API

## Decision Outcome

Chosen option: "required allowlist, scheme and address checks, two
credentials, per-IP limit".

Destination checks, applied in the domain and again as shorten policy.
Cleaning ([ADR-0008](0008-url-cleaning.md)) runs first and does not waive
these:

* Scheme is `http` or `https`. Reject `javascript:`, `data:`, and any
  other scheme.
* When private-host blocking is on (the supported configuration), reject
  loopback, private, link-local, and unspecified addresses using
  `net/netip`. Name resolution used for that check must not be treated
  as a fetch of the destination body.
* Host must be on `VEYLQO_ALLOWED_HOSTS`. The variable is required. An
  empty allowlist means shorten fails closed, not "allow the world".
* Hosts are compared as ASCII or punycode. Do not accept a Unicode label
  that has not been converted.
* A host that is an IP literal is allowed only if that address is on the
  allowlist and is not a blocked address. Private and loopback fail even
  if someone adds them to the allowlist. The block wins.

Credentials:

* The publishable key authorizes `POST /api/v1/links` only. It is safe
  to embed in a page. It is not an admin credential.
* The admin token authorizes `GET` and `DELETE` of a link and stats.
  It is distinct from the publishable key. Knowing a public code is not
  admin access ([ADR-0006](0006-internal-uuidv7-and-public-base62.md)).
* Missing or wrong publishable key on create is 401.
* Missing or wrong admin token on admin routes is 401.
* CORS allows only configured origins. Admin routes also use
  `http.CrossOriginProtection` (VYQ-35).

Rate limit:

* Per-IP token bucket.
* Client IP is taken from the connection, unless the peer is in
  `VEYLQO_TRUSTED_PROXIES`, in which case a forwarded header may be used.
  Untrusted forwarded headers are ignored.
* Excess requests are 429.
* The limiter does not write an IP log. In-memory buckets are not click
  analytics ([ADR-0009](0009-async-clicks-and-privacy.md)).

`GET /{code}` does not require the publishable key. It redirects only to
a destination that already passed these checks at create time. It is not
a general URL proxy: the code is looked up, and the stored destination
is sent in `Location`. The handler must not request the destination itself.

No new `require` is authorized for abuse tooling.

### Consequences

* Good, because an unconfigured server cannot be used as an open redirect.
* Good, because the browser key cannot delete links or read stats.
* Bad, because operators must list every destination host they intend to
  share. That is deliberate friction.
* Bad, because a DNS rebinding or a host that later starts resolving to
  a private address is only caught if the address check runs at shorten
  time against the lookup result. We do not re-fetch on every redirect.
  Redirect still only emits the stored URL; it does not connect to it.
* Neutral, because rate limits are per process. Multiple replicas do not
  share a bucket unless a later ADR adds a shared limiter.

### Confirmation

* VYQ-15: `javascript:alert(1)` and `http://127.0.0.1/` (blocking on) fail.
* VYQ-20: a host outside the allowlist is `incorrect-input`.
* VYQ-35: limiter and auth middleware tests pass; `POST /api/v1/links`
  without the publishable key is 401.
* VYQ-42: the README names `VEYLQO_ALLOWED_HOSTS`.

## Pros and Cons of the Options

### Allowlist, scheme and address checks, two keys, per-IP limit

* Good, because it matches `AGENTS.md` and fails closed.
* Good, because it uses `net/netip` and the standard library CSRF helper,
  not a new module.
* Bad, because host lists are operationally annoying, especially for
  user-generated destinations. That use case is out of scope until a
  later ADR.

### Authenticated, but any public host

* Good, because operators would not maintain a host list.
* Bad, because a leaked publishable key would still create redirects to
  arbitrary sites. Rejected: the key is designed to be public in a page.

### Private-host block only

* Good, because it stops the obvious SSRF destinations.
* Bad, because the service would still be a public open redirect onto
  any internet host. Rejected.

### CAPTCHA or external abuse API

* Good, because it can slow anonymous abuse.
* Bad, because it adds a third party and a `require`, and it does not
  replace the allowlist. Not chosen.

## More Information

* URL rewriting must not weaken these checks:
  [ADR-0008](0008-url-cleaning.md).
* 302 behavior for an already stored link:
  [ADR-0007](0007-temporary-302-redirects.md).
* Contract status codes belong in `api/openapi/veylqo.json` when VYQ-31
  is done, not only in this note.
