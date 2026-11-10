---
id: VYQ-4
type: epic
title: F3 — Adapters
phase: F3
depends_on: [VYQ-3]
updated: 2026-11-09
---

# VYQ-4 — F3 — Adapters

## Goal

Persistence and side effects: memory, SQLite, Postgres, code generator, click batcher, and LRU resolve cache.

## Done criteria

The shared repository suite passes on memory, SQLite, and Postgres. Drivers imported only in `cmd/veylqo` (once it exists).
