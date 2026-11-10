---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0011: Separate backend and JS repositories, one contract

Status: Accepted

## Context and Problem Statement

The link service is a Go binary. The share widget is a vanilla Web
Component with no runtime framework, published for a `<script>` tag or
npm. They release on different cadences, but they must not grow two
HTTP contracts. The board already tracks JS work in this repository
while the code lives elsewhere.

Where does each tree live, and how does a contract change reach the SDK?

## Decision Drivers

* Backend remote: `git@github.com-unisimon:jovrion/veylqo.git`,
  module `github.com/jovrion/veylqo` (VYQ-11).
* SDK remote: `git@github.com-unisimon:jovrion/veylqo-js.git`
  (`AGENTS.md`, VYQ-43).
* JS tasks are area `js`. Implementation runs in `jovrion/veylqo-js`.
  Evidence is recorded on the task in this repo.
* The contract is `api/openapi/veylqo.json` in this repo
  ([ADR-0004](0004-openapi-json-without-codegen.md)).
* The SDK has no UI runtime dependencies (VYQ-7, VYQ-44).
* No Go codegen, and no new Go `require` for the SDK.

## Considered Options

* Two repositories, spec in the Go repo, JS client derived from that spec
* One monorepo for Go and the Web Component
* SDK hand-written against examples, spec optional
* Generate the Go server and the JS client from a third repository

## Decision Outcome

Chosen option: "two repositories, spec in the Go repo, JS client derived
from that spec".

Repositories:

| Tree | Remote | What it contains |
| --- | --- | --- |
| This repo | `git@github.com-unisimon:jovrion/veylqo.git` | Go service, OpenAPI spec, board, ADRs |
| SDK | `git@github.com-unisimon:jovrion/veylqo-js.git` | `core`, `element`, `cdn` packages |

Tracking stays here. A `js` task is claimed and closed in this board.
The code change and its tests happen in `jovrion/veylqo-js`. Do not
implement the SDK under this repository's `cmd/` or `internal/`.

Contract sync:

1. Change `api/openapi/veylqo.json` in this repository first
   ([ADR-0004](0004-openapi-json-without-codegen.md)).
2. Land the Go handler and the contract test in this repository.
3. In `jovrion/veylqo-js`, regenerate the client types from that spec.
   `openapi-typescript` may be a dev dependency there. It must not be a
   runtime dependency, and it must not be added to this repo's `go.mod`.
4. Hand-written SDK behavior (`shorten()`, `share()`, `<veylqo-share>`)
   calls that client. It does not fork the path list.

The publishable key ([ADR-0010](0010-abuse-prevention.md)) is the
credential the element may embed. The admin token must not appear in
SDK examples or in browser code.

SDK shape, accepted as constraints for later tasks, not implemented here:

* pnpm workspaces, packages `core`, `element`, and `cdn` (VYQ-43).
* `@veylqo/core` has no UI runtime dependencies (VYQ-44).
* `<veylqo-share>` is a vanilla custom element (VYQ-45).
* Examples for static HTML, React, Vue, and Svelte consume the element;
  they do not reimplement shorten (VYQ-47).

License for both trees is MIT, added by their own setup tasks. This ADR
does not add `LICENSE`.

### Consequences

* Good, because the widget can be versioned and published without a Go release.
* Good, because agents have one board and one spec.
* Bad, because a contract change is two pull requests, and the second
  can lag. Until the SDK is regenerated, do not claim the client matches.
* Bad, because evidence for `js` tasks lives in another working tree.
  The task file in this repo must still record the command that passed.
* Neutral, because issues and ADRs stay with the backend repo. The JS
  repo gets its own `AGENTS.md` and does not keep a second decision log
  for HTTP semantics.

### Confirmation

* VYQ-31: the spec file exists in this repo and parses as JSON.
* VYQ-43: `jovrion/veylqo-js` has `pnpm-workspace.yaml` and
  `packages/{core,element,cdn}`.
* VYQ-44: `@veylqo/core` tests pass and its `package.json` lists no UI
  runtime dependencies.
* A `js` task is not closed from Go tests alone. Evidence comes from the
  SDK repo.

## Pros and Cons of the Options

### Two repositories, one spec in the Go repo

* Good, because it matches the remotes already named in `AGENTS.md`.
* Good, because npm publish does not require the Go module.
* Bad, because cross-repo changes are not atomic.

### Monorepo

* Good, because one commit could update the spec and the client.
* Bad, because the board and the remotes already split them, and the JS
  toolchain should not live in the Go module.

### Hand-written client, spec optional

* Good, because the first widget would not wait on a generator.
* Bad, because the client would drift, which is the failure this ADR exists to prevent.

### Third repository for the contract

* Good, because neither implementation would "own" the spec.
* Bad, because every change would touch three trees. The service is the
  authority for its HTTP API. The spec stays next to the handlers.

## More Information

* Spec format and the ban on Go codegen:
  [ADR-0004](0004-openapi-json-without-codegen.md).
* Which key the browser may hold:
  [ADR-0010](0010-abuse-prevention.md).
* `builder-js` implements area `js`. `builder-go` does not edit the SDK
  repo as if it were `internal/`.
