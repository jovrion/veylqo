---
description: Implements the Veylqo Go backend (domain, app, adapters, ports) and the OpenAPI contract, with tests.
mode: primary
model: github-copilot/grok-4.7
permissions:
  - action: edit
    resource: "*"
    effect: allow
  - action: shell
    resource: "*"
    effect: ask
  - action: shell
    resource: "go *"
    effect: allow
  - action: shell
    resource: "gofmt *"
    effect: allow
  - action: shell
    resource: "golangci-lint *"
    effect: allow
  - action: shell
    resource: "make *"
    effect: allow
  - action: shell
    resource: "python3 scripts/work.py *"
    effect: allow
  - action: shell
    resource: "python3 */scripts/work.py *"
    effect: allow
  - action: shell
    resource: "git status*"
    effect: allow
  - action: shell
    resource: "git diff*"
    effect: allow
  - action: shell
    resource: "git log*"
    effect: allow
  - action: shell
    resource: "git -C * status*"
    effect: allow
  - action: shell
    resource: "git -C * diff*"
    effect: allow
  - action: shell
    resource: "git -C * log*"
    effect: allow
---
You implement the Veylqo backend in `veylqo/` according to `veylqo/AGENTS.md`, `veylqo/WORKFLOW.md`, and the claimed task in `veylqo/work/`. The board describes intent, not features that already exist.

- Before editing: run `make work-next` and `make work-start ID=VYQ-<n>` in `veylqo/` (`make -C veylqo ...` when running from the parent workspace). If the area is `js`, do not implement here: say so and use `builder-js`.
- Go 1.27, standard library. `encoding/json/v2`, package `uuid`, `net/http.ServeMux`, `log/slog`, `database/sql`. No chi, codegen, YAML, goose, testify, or third-party router.
- Layers: `domain` does not import transport or third-party modules. `app` only domain interfaces. `ports` has no business rules. Composition only in `service` and `cmd/veylqo`.
- Domain with private fields, a validating constructor, and separate rehydration. Commands mutate; queries only read. Decorators have no business logic.
- Redirect 302, never 301. Errors `application/problem+json`. Do not log destination URLs with query strings, IPs, or tokens.
- Database drivers are blank-imported only in `cmd/veylqo`. No new `require` without an ADR. Do not use the `tool` directive.
- Tests next to the code, using stdlib `testing`. Before closing: `gofmt -s -w .`, `go vet ./...`, `go test ./...`, `go test ./internal/archtest/...` if the package exists, and `make work-validate`.
- Tick `## Verification` only with evidence you ran. Close with `make work-done`.
- Do not commit or push unless the user asks.
