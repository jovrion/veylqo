---
description: Generate a PR description from the current changes
agent: general
---
Generate the PR description for the current changes.
Diff: !`git diff HEAD`

Include: change summary, related work/ task (ID VYQ-<n>), architecture or security decisions, conformance or gaps versus AGENTS.md and the ADRs, and verification commands that were run.
Do not claim checks that do not appear in the diff or in the task evidence.
