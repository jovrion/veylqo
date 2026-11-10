---
id: VYQ-5
type: epic
title: F4 — Ports and contract
phase: F4
depends_on: [VYQ-3]
updated: 2026-11-09
---

# VYQ-5 — F4 — Ports and contract

## Goal

OpenAPI 3.1 JSON as the source of truth, HTTP with ServeMux, 302 redirects, and abuse controls for public shortening.

## Done criteria

The contract test fails if spec and handlers diverge. `GET /{code}` responds 302, never 301.
