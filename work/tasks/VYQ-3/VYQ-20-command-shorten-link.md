---
id: VYQ-20
type: task
title: "[go] command ShortenLink"
status: "done"
priority: "critical"
area: "go"
epic: VYQ-3
depends_on: [VYQ-16, VYQ-17, VYQ-18]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-20 — [go] command ShortenLink

## Scope

Clean the URL, check the host allowlist, reuse the code if the URL already exists and the link has neither a custom slug nor expiry, otherwise generate a code and retry on collision. Idempotent.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/app/command/ -run Shorten` green
- [x] two shortenings of the same URL without a slug return the same code
- [x] a host outside the allowlist is `incorrect-input`

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/app/command/ -run Shorten -count=1
ok  github.com/jovrion/veylqo/internal/links/app/command
go test ./...
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/common/errors
ok  github.com/jovrion/veylqo/internal/links/app/command
ok  github.com/jovrion/veylqo/internal/links/app/query
ok  github.com/jovrion/veylqo/internal/links/domain/link
go test ./internal/archtest/...
ok  github.com/jovrion/veylqo/internal/archtest
```

`TestShorten/same_URL_without_a_slug_returns_the_same_code` shortens `https://Example.COM/a?utm_source=one` and `https://example.com/a?utm_source=two&fbclid=abc` and both return `Ab3xyZ9`. The generator and `Add` run once. The stored destination has no tracking query. The result is the public code, not the UUIDv7.

`TestShorten/host_outside_the_allowlist_is_incorrect-input` rejects `evil.test` when the list is `example.com`. Slug is `incorrect-input`. `Add` and the generator are not called. The error text does not include the host or the query. An empty allowlist fails closed the same way. A private host still fails destination validation when it is on the list.

Reuse is skipped when the request has a custom slug or an expiry, and when the stored link has an expiry. A code conflict retries with the next generated code. A destination conflict on an idempotent request re-reads and returns the winner. `clean: false` does not collapse tracking parameters. The command does not import `net/http`, `database/sql`, or `log`/`log/slog`. Code minting stays behind `CodeGenerator`; the `crypto/rand` generator remains VYQ-30.
