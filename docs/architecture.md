# Architecture

Self-hosted link shortener. Tagline: Less URL. More destination.

This document is the map. Decisions live in [`docs/adr/`](adr/).
Do not copy them here. A change of decision needs a new ADR
([ADR-0001](adr/0001-record-architecture-decisions.md)), not a silent
edit of this file.

The board describes intent. A layer, flow, or control named below is
not shipped until its task is `done`. Nothing in this document is a
claim that the binary already shortens or redirects.

## Layers

One bounded context, `internal/links`. Composition is not a sixth
business layer. Rules and the import graph:
[ADR-0002](adr/0002-clean-architecture-and-wild-workouts.md).

```text
                    cmd/veylqo
                    config, drivers, migrations, shutdown
                           |
                           v
                    service          wires only; no use-case logic
                           |
           +---------------+---------------+
           |                               |
           v                               v
        ports                            adapters
        HTTP edge                        SQL, batcher, cache, codes
        no business rules                implement domain interfaces
           |                               |
           v                               v
        app                              app
        commands mutate                  queries only read
        decorators: recovery → logging → metrics
           |
           v
        domain
        entities, value objects, repository interfaces
        no HTTP, no SQL, no third-party modules
```

| Layer | Package | Owns | Must not |
| --- | --- | --- | --- |
| `domain` | `internal/links/domain` | `Link`, `Code`, destination, cleaner, repository interfaces | Import `app`, `adapters`, `ports`, `service`, or a third-party module |
| `app` | `internal/links/app` | Commands and queries against domain interfaces | Import `net/http` or SQL; contain decorator business rules |
| `adapters` | `internal/links/adapters` | Memory, SQLite, Postgres, code generator, click batcher, resolve cache | Import a database driver; invent domain policy |
| `ports` | `internal/links/ports` | `net/http.ServeMux` handlers, problem+json mapping | Decide allowlists, expiry, or cleaning |
| `service` | `internal/links/service` | `NewApplication`: wire adapters into `app` | Hold use-case logic |

`cmd/veylqo` is the process entrypoint. Database drivers are
blank-imported only there ([ADR-0003](adr/0003-stdlib-policy-and-driver-exceptions.md)).

`app.Application` exposes `Commands` and `Queries`. Commands: Shorten,
RecordClick, Delete. Queries: Resolve, Get, Stats. Resolve does not
record a click. Handlers leave the application constructor already
wrapped. Decorator order is recovery, then logging, then metrics.

## Shorten flow

`POST /api/v1/links`. Spec and status codes:
[ADR-0004](adr/0004-openapi-json-without-codegen.md).
Abuse checks: [ADR-0010](adr/0010-abuse-prevention.md).

```text
client
  |  publishable key, not the admin token
  v
ports          rate limit, auth, JSON (encoding/json/v2, RejectUnknownMembers)
  v
app.Shorten    command; mutates
  v
domain         clean, then validate, then allowlist
  |            http/https only; private and loopback blocked
  v
adapters       insert by public code; retry on unique conflict
  v
ports          201 with the public code, or problem+json
```

Order inside the command, not in the handler:

1. Clean the destination ([ADR-0008](adr/0008-url-cleaning.md)).
   `clean: false` skips tracking-parameter stripping only. It does not
   skip scheme checks, fragment dropping, private-host blocking, or the
   allowlist.
2. Reject anything that is not `http` or `https`, and reject private
   and loopback hosts. The block wins even if the host was added to
   `VEYLQO_ALLOWED_HOSTS`.
3. Require the host on `VEYLQO_ALLOWED_HOSTS`. Empty allowlist fails
   closed. Hosts are ASCII or punycode. The standard library does not
   normalize IDN.
4. Allocate the public code ([ADR-0006](adr/0006-internal-uuidv7-and-public-base62.md)).
   Internal key is `uuid.NewV7()`. The public code is base62, length 7,
   or a custom slug of 3–64 characters. They are not the same string.
   Reserved names (`api`, `healthz`, `readyz`, `metrics`, `debug`,
   `static`) cannot be codes.
5. Insert. On a unique conflict, generate another random code and retry.
   Idempotent reuse of an existing code applies only when the cleaned
   destination matches and the existing link has neither a custom slug
   nor an expiry. The response returns the public code, never the UUID.

The handler does not fetch the destination. Storing a URL is not a
request to that URL.

## Resolve flow

`GET /{code}` is outside `/api`. Status rules:
[ADR-0007](adr/0007-temporary-302-redirects.md).
Click recording: [ADR-0009](adr/0009-async-clicks-and-privacy.md).

```text
browser  GET /{code}
  v
ports    reserved routes never fall through (/api, /healthz, /readyz, /metrics)
  v
app.Resolve     query; reads only; does not record a click
  v
domain / cache  public code only; a UUID is not accepted on this path
  |
  +-- missing ---------------> 404 application/problem+json
  +-- expired or deleted ----> 410 application/problem+json
  |                            do not redirect to the old destination
  +-- active
        v
      ports  302 Location: stored destination
             Cache-Control: private, max-age=0
             never 301
        |
        +-- RecordClick is invoked and not awaited
              full buffer drops the click; the 302 still returns
```

The redirect status does not depend on whether the click was queued.
`RecordClick` hands the event to an in-process batcher. A click is a
count for a public code, not a stored HTTP request. If the process dies
before flush, those clicks are gone. Analytics are approximate. Do not
promise exactly-once.

A resolve cache may hold the read model. It must not emit a 301, and
delete must drop the entry. A cache hit is still a **302**.

## Security limits

Public shortening is not an open redirect
([ADR-0010](adr/0010-abuse-prevention.md)).

| Control | Rule |
| --- | --- |
| Allowlist | `VEYLQO_ALLOWED_HOSTS` is required. Empty means shorten fails closed. |
| Scheme | `http` and `https` only. `javascript:` and `data:` are rejected. |
| Addresses | Loopback, private, link-local, and unspecified addresses are blocked. The check is not a fetch of the destination. |
| Credentials | Publishable key authorizes `POST /api/v1/links` only. Admin token authorizes get, delete, and stats. They are distinct. Knowing a code is not admin access. |
| Rate limit | Per-IP token bucket. Client IP comes from the connection, unless the peer is in `VEYLQO_TRUSTED_PROXIES`. Excess is 429. The limiter is not a visit log. |
| Redirect | `GET /{code}` needs no publishable key. It emits the stored destination. It does not connect to it. |
| Errors | `application/problem+json` (RFC 9457). Ports map slugs to 400, 401, 404, 409, 410, and 429. |
| Logs | Do not log destination URLs with query strings, client IPs, or tokens. Metrics may count status classes and dropped clicks. They must not label a series with a URL, a raw IP, or a token. |
| CORS | Configured origins only. Admin routes also use `http.CrossOriginProtection`. |

DNS rebinding after shorten is not re-checked on every redirect. The
redirect still only sends the stored URL. It does not open a connection
to the host.

## Storage and contract

Persistence is `database/sql` behind domain interfaces, with memory,
SQLite, and Postgres adapters
([ADR-0005](adr/0005-postgres-and-sqlite-via-database-sql.md)).
SQLite is one writer. It is the small-install engine. It does not
replace Postgres under load. Migrations are embedded SQL, not goose.
Drivers are not imported by adapter packages.

The HTTP contract is `api/openapi/veylqo.json` (OpenAPI 3.1 JSON, no
YAML, no codegen). Spec first, then the handler, then the contract test
([ADR-0004](adr/0004-openapi-json-without-codegen.md)).

The JS SDK lives in `jovrion/veylqo-js`. Tasks are tracked here; code
is not ([ADR-0011](adr/0011-separate-repos-and-contract-sync.md)).

## Out of the MVP

Rejected by an accepted ADR, or explicitly deferred. Do not add them
without a new ADR.

- Permanent redirects (301) and method-preserving 307 or 308. The public
  redirect is 302 only.
- HTML or JavaScript bounce pages and interstitial warnings.
- Open shortening to any public host, including "authenticated but any
  host" and "block private hosts only".
- CAPTCHA or an external abuse API.
- Fetching or following the destination at shorten or resolve time.
- Raw click logs: IP, user agent, referrer, cookie, token, or destination
  URL. Exactly-once delivery, an outbox, or a broker.
- Synchronous click inserts on the redirect path.
- chi, Watermill, a message bus, YAML config, goose, testify, protobuf,
  or a generated OpenAPI server.
- A third-party UUID, shortid, or URL-normalization library.
- IDN normalization. Unicode hosts that are not already punycode are
  rejected, not rewritten.
- Using SQLite as a multi-writer or high-load store.
- A shared rate-limit bucket across replicas.
- User-generated destinations with no operator allowlist.
- Putting the UUIDv7, or a sequential integer, in the public path.
- Implementing the Web Component inside this repository.

## ADR index

| ADR | Use it for |
| --- | --- |
| [0001](adr/0001-record-architecture-decisions.md) | How decisions are recorded |
| [0002](adr/0002-clean-architecture-and-wild-workouts.md) | Layers, commands, queries, decorators |
| [0003](adr/0003-stdlib-policy-and-driver-exceptions.md) | Standard library and the two driver exceptions |
| [0004](adr/0004-openapi-json-without-codegen.md) | Contract shape and change order |
| [0005](adr/0005-postgres-and-sqlite-via-database-sql.md) | Storage engines and migrations |
| [0006](adr/0006-internal-uuidv7-and-public-base62.md) | Internal key and public code |
| [0007](adr/0007-temporary-302-redirects.md) | 302, never 301 |
| [0008](adr/0008-url-cleaning.md) | Tracking-parameter cleaning |
| [0009](adr/0009-async-clicks-and-privacy.md) | Non-blocking counts, no request logs |
| [0010](adr/0010-abuse-prevention.md) | Allowlist, keys, rate limit |
| [0011](adr/0011-separate-repos-and-contract-sync.md) | Backend repo and JS repo |
