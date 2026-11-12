# Contributing

Veylqo tracks work on a local board in this repository. A task file is
intent. Do not treat a feature as present unless its task is `done`.

## Board

```
make work-next
make work-start ID=VYQ-<n>
make work-done  ID=VYQ-<n>
make work-validate
```

Statuses are `open`, `in-progress`, `blocked`, and `done`. At most one
task is in progress. Tick a `## Verification` box only after you ran
the check. `make work-done` refuses to close an open box.

Rules: `WORKFLOW.md`, `work/README.md`, `AGENTS.md`. Design:
`docs/architecture.md` and `docs/adr/`. An architecture change needs a
new ADR. Do not invent a parallel design.

Areas `go`, `contract`, `setup`, `ops`, and `docs` land in this repo.
Area `js` is implemented in
[`jovrion/veylqo-js`](https://github.com/jovrion/veylqo-js). Record the
evidence on the task here.

## Code

Go 1.27, standard library first. No new `require` in `go.mod` without
an ADR. Do not use the `tool` directive. Database drivers are
blank-imported only in `cmd/veylqo`.

Layers live under `internal/links`. `domain` does not import transport
or third-party modules. `app` only knows domain interfaces. `ports`
has no business rules. Composition belongs in `internal/links/service`
and `cmd/veylqo`.

HTTP uses `net/http.ServeMux`. Request JSON uses `encoding/json/v2`
with unknown members rejected. Errors are `application/problem+json`.
Redirects are 302, never 301.

Before you call a change done:

```
gofmt -s -w .
go vet ./...
go test ./...
go test ./internal/archtest/...
make work-validate
```

## Secrets and privacy

Do not commit `.env`, database files, or real keys. Compose placeholders
are not production secrets.

Do not log destination URLs with query strings, client IPs, or tokens.
Click analytics are counts. They are approximate: a crash before flush
loses queued clicks. Do not promise exactly-once.

Do not add an open redirect, a fetch of the destination, or a path that
treats the publishable key as the admin token.
