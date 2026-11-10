---
id: VYQ-2
type: epic
title: F1 — Domain
phase: F1
depends_on: [VYQ-1]
updated: 2026-11-09
---

# VYQ-2 — F1 — Domain

## Goal

Model the `links` bounded context with no infrastructure: codes, destinations, URL cleaning, and the Link entity.

## Done criteria

`go test ./internal/links/domain/...` green. The package imports nothing outside the standard library.
