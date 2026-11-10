---
id: VYQ-6
type: epic
title: F5 — Service and distribution
phase: F5
depends_on: [VYQ-4, VYQ-5]
updated: 2026-11-09
---

# VYQ-6 — F5 — Service and distribution

## Goal

One self-hosted binary: wiring, env config, Docker, e2e CI, and an operations README.

## Done criteria

`docker compose up` shortens and resolves against SQLite or Postgres. `/healthz` and `/readyz` respond.
