---
id: VYQ-18
type: task
title: "[go] SlugError in internal/common/errors"
status: "done"
priority: "major"
area: "go"
epic: VYQ-3
depends_on: [VYQ-12]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-18 — [go] SlugError in internal/common/errors

## Scope

Errors with a slug: `incorrect-input`, `not-found`, `gone`, `conflict`, `unauthorized`, `rate-limited`. Translated to HTTP in `ports`, not here. Wrapping tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/common/errors/` green
- [x] each documented slug has a constructor

## Evidence & Notes

- `go test ./internal/common/errors/` — ok (also `gofmt -s -w .`, `go vet ./...`, `go test ./...`, `go test ./internal/archtest/...`).
- Constructors: `NewIncorrectInput`, `NewNotFound`, `NewGone`, `NewConflict`, `NewUnauthorized`, `NewRateLimited`. `TestEachDocumentedSlugHasAConstructor` checks each slug.
- HTTP status mapping is not in this package. `TestPackageDoesNotImportHTTP` rejects `net/http` and `database/sql`.
- `Error()` omits the wrapped cause and redacts a message that contains `://`, `?`, or a newline.
