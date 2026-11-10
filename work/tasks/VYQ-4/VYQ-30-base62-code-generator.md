---
id: VYQ-30
type: task
title: "[go] crypto/rand base62 code generator"
status: "open"
priority: "major"
area: "go"
epic: VYQ-4
depends_on: [VYQ-14]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-30 — [go] crypto/rand base62 code generator

## Scope

Generator with `crypto/rand` and rejection sampling (unbiased). Default length 7. Alphabet and trivial non-collision tests.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test` of the generator is green
- [ ] 1000 consecutive codes have length 7 and a base62 alphabet

## Evidence & Notes

_Pending._
