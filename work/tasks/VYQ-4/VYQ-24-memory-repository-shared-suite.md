---
id: VYQ-24
type: task
title: "[go] In-memory repository and shared suite"
status: "open"
priority: "critical"
area: "go"
epic: VYQ-4
depends_on: [VYQ-17]
blocked_reason: null
updated: 2026-11-09
---

# VYQ-24 — [go] In-memory repository and shared suite

## Scope

`link_memory_repository.go` plus a test suite every adapter must pass: Add, Get, GetByDestination, atomic Update, code collision, delete.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `go test ./internal/links/adapters/ -run Memory` green
- [ ] the suite lives in a file reused by SQLite and Postgres

## Evidence & Notes

_Pending._
