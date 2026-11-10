---
status: "{proposed | rejected | accepted | deprecated | superseded by ADR-NNNN}"
date: "{YYYY-MM-DD when the decision was last updated}"
decision-makers: "{who accepted the decision}"
---

# {short title, problem and solution}

Status: Proposed | Accepted | Rejected | Deprecated | Superseded by [ADR-NNNN](NNNN-kebab-title.md)

One decision per file. File name: `NNNN-kebab-case-title.md`. English.
Copy this file, replace every `{placeholder}`, and set Status to Accepted
only when the decision is chosen. Do not leave the template text in an
accepted ADR.

An accepted ADR records a decision. It does not claim that product code
already exists. Implementation stays on the board in `work/`.

## Context and Problem Statement

{What forces the decision? Prefer a question. Two or three sentences.
Link the board task that will implement it. Do not describe unfinished
work as shipped.}

## Decision Drivers

* {force or constraint, for example a rule already fixed in AGENTS.md}
* {force or constraint}

## Considered Options

* {option 1}
* {option 2}
* {option 3}

## Decision Outcome

Chosen option: "{option 1}", because {justification against the drivers}.

### Consequences

* Good, because {positive consequence}
* Bad, because {cost or limit we accept}
* Neutral, because {something that does not change}

### Confirmation

{How a later change can show the decision still holds: a path, a test
name, or a board task. Do not tick work that has not been run.}

## Pros and Cons of the Options

### {option 1}

* Good, because {argument}
* Bad, because {argument}

### {option 2}

* Good, because {argument}
* Bad, because {argument}

## More Information

* Supersede by writing a new ADR. Mark this one
  `Superseded by ADR-NNNN`. Do not silently rewrite an accepted decision
  into a different one.
* Related: [ADR-0001](0001-record-architecture-decisions.md).
* A design narrative belongs in `docs/architecture.md` (VYQ-10) and must
  point here instead of copying the decision.
