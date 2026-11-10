---
description: Write tests for a package or file
agent: builder-go
---
Write tests for: $ARGUMENTS
Before implementing, follow the AGENTS.md task cycle. Use only stdlib `testing`. Cover the invariant the package promises (domain validation, layers, HTTP contract, shortening idempotency). Run `go test ./...`.
