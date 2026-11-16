---
description: Write tests for a package or file
agent: builder-go
---
Write tests for: $ARGUMENTS
Before implementing, follow the `AGENTS.md` task cycle (`veylqo/work/`). If the target is in `veylqo-js/`, say so and switch to `builder-js`. Use only stdlib `testing`. Cover the invariant the package promises (domain validation, layers, HTTP contract, shortening idempotency). Run `go test ./...` in `veylqo/`.
