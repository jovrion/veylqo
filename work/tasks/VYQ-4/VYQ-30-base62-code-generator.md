---
id: VYQ-30
type: task
title: "[go] crypto/rand base62 code generator"
status: "done"
priority: "major"
area: "go"
epic: VYQ-4
depends_on: [VYQ-14]
blocked_reason: null
updated: 2026-11-11
---

# VYQ-30 — [go] crypto/rand base62 code generator

## Scope

Generator with `crypto/rand` and rejection sampling (unbiased). Default length 7. Alphabet and trivial non-collision tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test` of the generator is green
- [x] 1000 consecutive codes have length 7 and a base62 alphabet

## Evidence & Notes

2026-11-11:

`adapters.CodeGenerator` reads `crypto/rand` and discards bytes in `[248, 255]` so each of the 62 symbols has four sources. Length is `link.RandomCodeLength` (7). The alphabet is `link.Base62Alphabet`; the adapter does not define another and does not emit `_` or `-`. It does not read the store and does not mint the UUID. Reserved names stay in `link.NewCode`; shorten already retries those. Not wired into `cmd` or `service` (VYQ-37).

```
gofmt -s -w .
go vet ./...
go test ./internal/links/adapters/ -count=1 -run CodeGenerator -v
--- PASS: TestCodeGeneratorThousandCodes (0.00s)
--- PASS: TestCodeGeneratorRejectionSampling (0.00s)
--- PASS: TestCodeGeneratorStopsWhenEntropyIsOnlyRejected (0.00s)
--- PASS: TestCodeGeneratorRejectsContext (0.00s)
--- PASS: TestCodeGeneratorReadError (0.00s)
ok  github.com/jovrion/veylqo/internal/links/adapters
go test ./...
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/common/decorator
ok  github.com/jovrion/veylqo/internal/common/errors
ok  github.com/jovrion/veylqo/internal/links/adapters
ok  github.com/jovrion/veylqo/internal/links/app
ok  github.com/jovrion/veylqo/internal/links/app/command
ok  github.com/jovrion/veylqo/internal/links/app/query
ok  github.com/jovrion/veylqo/internal/links/domain/link
ok  github.com/jovrion/veylqo/internal/migrate
go test ./internal/archtest/...
ok  github.com/jovrion/veylqo/internal/archtest
golangci-lint run ./internal/links/adapters/...
0 issues.
```

`TestCodeGeneratorThousandCodes` draws 1000 codes from `crypto/rand`. Each has length 7 and stays in `Base62Alphabet`. The set has more than one distinct value. `TestCodeGeneratorRejectionSampling` feeds 255 and 248 before each accepted byte and checks they are consumed, not reduced onto the alphabet.
