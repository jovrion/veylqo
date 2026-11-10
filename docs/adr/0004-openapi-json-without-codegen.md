---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0004: OpenAPI JSON contract, no codegen

Status: Accepted

## Context and Problem Statement

The HTTP API is the boundary for the self-hosted binary and for the JS
SDK in another repository. Generated servers drift, and YAML adds a
parser we do not want. The board already says the contract is
`api/openapi/veylqo.json` and that a change starts in the spec.

How is the contract written, and how do handlers stay aligned with it
without a code generator?

## Decision Drivers

* OpenAPI 3.1, JSON, no YAML (`AGENTS.md`).
* Spec first, then the handler, then a contract test that must pass.
* No generated Go files to edit by hand.
* `net/http.ServeMux` only ([ADR-0003](0003-stdlib-policy-and-driver-exceptions.md)).
* Errors are `application/problem+json` (RFC 9457).
* The JS client must follow the same document
  ([ADR-0011](0011-separate-repos-and-contract-sync.md)).

## Considered Options

* Hand-written OpenAPI 3.1 JSON, hand-written handlers, contract test
* OpenAPI YAML with a Go server generator
* No spec; handlers and a README are the contract
* A Go framework that generates the spec from comments

## Decision Outcome

Chosen option: "hand-written OpenAPI 3.1 JSON, hand-written handlers,
contract test".

The source of truth is `api/openapi/veylqo.json`. It is JSON so the
standard library can read it. There is no YAML twin.

Planned operations (VYQ-31; not implemented by this ADR):

| Method and path | Role |
| --- | --- |
| `POST /api/v1/links` | Shorten |
| `GET /api/v1/links/{code}` | Fetch one link |
| `DELETE /api/v1/links/{code}` | Delete |
| `GET /api/v1/links/{code}/stats` | Counts |
| `GET /healthz` | Process up |
| `GET /readyz` | Ready to serve |
| `GET /{code}` | Redirect, outside `/api` |

`GET /{code}` is documented as **302**, never 301. Expired links are
documented as 410. See [ADR-0007](0007-temporary-302-redirects.md).

Error responses use `application/problem+json`. Ports map application
slugs to 400, 401, 404, 409, 410, and 429. Ports do not invent extra
success codes that the spec omits.

Change order, every time:

1. Edit `api/openapi/veylqo.json`.
2. Implement or adjust the hand-written `ServeMux` handler.
3. The contract test must pass.

The contract test reads the spec with `encoding/json/v2` and checks that
every spec path and response code has a handler, and that the mux has no
undocumented route. Removing a path from either side fails the test
(VYQ-36).

No Go generator. Do not commit `openapi-generator`, `oapi-codegen`, or
similar output, and do not add those tools to `go.mod`. The JS repository
may use `openapi-typescript` as a **dev** dependency to derive a client
from this file. That output is not a second source of truth, and it is
not a Go `require`. See [ADR-0011](0011-separate-repos-and-contract-sync.md).

### Consequences

* Good, because a contract change is a JSON diff anyone can review.
* Good, because handlers cannot grow an undocumented route without
  failing the contract test.
* Bad, because request and response types are written twice: once in the
  spec, once in Go. The test checks paths and status codes, not every
  schema field, unless a later task extends it.
* Bad, because OpenAPI 3.1 features must be written carefully by hand.

### Confirmation

* VYQ-31: `api/openapi/veylqo.json` parses as JSON, documents 302 for
  `GET /{code}`, and documents 410 for expired links.
* VYQ-32: unknown JSON members are rejected; errors are problem+json;
  no chi.
* VYQ-36: the contract test fails if a path disappears from the spec or
  from the mux.

## Pros and Cons of the Options

### Hand-written JSON spec, hand-written handlers, contract test

* Good, because it uses no generator and no YAML parser.
* Good, because spec-first matches how the JS SDK consumes the API.
* Bad, because boilerplate is manual.

### YAML plus a Go server generator

* Good, because handlers and types would be generated.
* Bad, because YAML is forbidden, generated files become the thing people
  edit, and a generator is a `require` this ADR does not allow.

### No spec

* Good, because there is nothing to keep in sync at the start.
* Bad, because the other repository would invent its own client, and
  agents would not have a single contract.

### Spec generated from Go comments

* Good, because the handler would be the only edit.
* Bad, because the contract would follow the code, which is the reverse
  of "spec first", and comment parsers are extra tools.

## More Information

* Redirect semantics are not redefined here:
  [ADR-0007](0007-temporary-302-redirects.md).
* Abuse responses (401, 429) are specified in
  [ADR-0010](0010-abuse-prevention.md) and must appear in the spec when
  those routes are written.
