---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0001: Record architecture decisions in MADR

Status: Accepted

## Context and Problem Statement

Veylqo is built by agents against a local board. `AGENTS.md` already
states non-negotiable rules, and later tasks name specific ADRs
(ADR-0003 for driver exceptions, ADR-0008 for URL cleaning). Without a
single decision log, those rules will be re-decided in chat or copied
into `docs/architecture.md`.

How do we record an architecture decision so a later task can read it,
and so a change requires a new decision instead of a silent edit?

## Decision Drivers

* Agents must read the ADR named by a task before editing.
* An architecture change needs a new ADR (`AGENTS.md`).
* The board describes intent. An ADR must not be mistaken for shipped code.
* One format, English, easy to diff in git.
* VYQ-10 must point at ADRs rather than duplicate them.

## Considered Options

* MADR (Markdown Architectural Decision Records) in `docs/adr/`
* Michael Nygard's short ADR template only
* Decisions only in `AGENTS.md` and task files
* An external tracker or ADR tool

## Decision Outcome

Chosen option: "MADR in `docs/adr/`", because it keeps context, drivers,
options, and consequences in the repository, in a format agents can
follow without a new tool.

Rules:

* One decision per file. Names are `NNNN-kebab-case.md`, starting at
  `0001`. `0000-template.md` is the blank MADR form, not a decision.
* Language is English.
* A chosen decision has `Status: Accepted` in the body and
  `status: accepted` in front matter.
* The template may say `Status: Proposed | Accepted`. A proposed or
  rejected experiment is not an accepted decision.
* Supersede by adding a new ADR and marking the old one
  `Superseded by ADR-NNNN`. Do not rewrite an accepted decision into a
  different decision.
* An accepted ADR authorizes a design. It does not mean the code exists.
  Do not describe a feature as done unless its board task is `done`.
* Product code does not belong in an ADR. No new `require` is authorized
  except where a later ADR says so.

### Consequences

* Good, because VYQ-10, VYQ-16, and the driver tasks have a stable place
  to point.
* Good, because options we rejected stay visible.
* Bad, because a decision can drift from the code if confirmation is
  skipped. Each ADR names how to confirm it.
* Neutral, because `AGENTS.md` remains the agent entry point. It points
  here; it does not become the decision log.

### Confirmation

`docs/adr/0000-template.md` and `docs/adr/0001-*.md` through
`docs/adr/0011-*.md` exist. Each accepted ADR contains the line
`Status: Accepted`. `docs/architecture.md` is out of scope for this
decision (VYQ-10).

## Pros and Cons of the Options

### MADR in `docs/adr/`

* Good, because context, options, and outcome are required, not optional prose.
* Good, because it is Markdown, with no generator and no YAML toolchain.
* Bad, because the template is longer than a one-paragraph note.

### Nygard template only

* Good, because it is shorter (context, decision, status, consequences).
* Bad, because rejected options disappear, which is how agents re-litigate them.

### Decisions only in `AGENTS.md` and tasks

* Good, because agents already read `AGENTS.md`.
* Bad, because `AGENTS.md` would grow into a design document, and task
  files are intent, not a durable decision log.

### External tracker or ADR tool

* Good, because some tools add status workflows.
* Bad, because this repository has no external tracker, and a tool would
  be a new dependency with nothing to justify it.

## More Information

* Template: [0000-template.md](0000-template.md).
* MADR: <https://adr.github.io/madr/>.
* The eleven decisions accepted with this log are ADR-0002 through
  ADR-0011.
* Narrative design (layer diagram, shorten and resolve flows) is VYQ-10,
  and must link these files instead of copying them.
