---
description: Diagnose and fix a bug
agent: builder-go
---
Bug: $ARGUMENTS
Before implementing, follow the AGENTS.md task cycle. Read the ADR that applies and do not break its invariants.
1) Reproduce with a failing test. 2) Root cause, not the symptom. 3) Minimal fix. 4) Test green, `gofmt`, and `go vet`.
If the failure is redirect, expiry, abuse, or database and the cause is not obvious, use the debugger subagent first. Do not add dependencies.
