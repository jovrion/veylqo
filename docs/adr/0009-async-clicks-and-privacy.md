---
status: accepted
date: 2026-11-10
decision-makers: Veylqo maintainers
---

# ADR-0009: Async click counts, without request logs

Status: Accepted

## Context and Problem Statement

Operators want to know how often a short link is opened. The redirect
must stay fast, and a click log is a surveillance trail if it stores
IPs, tokens, or full destination URLs. The process can also die between
accepting a click and writing it.

What do we store, when do we write it, and what accuracy do we promise?

## Decision Drivers

* Click analytics are counts, not raw request logs (`AGENTS.md`).
* Do not log destination URLs with query strings, IPs, or tokens.
* `RecordClick` must not block the redirect
  ([ADR-0007](0007-temporary-302-redirects.md)).
* A full buffer drops the click and increments a metric (VYQ-28).
* Shutdown drains pending events (VYQ-28, VYQ-38).
* A crash before flush loses clicks. Do not promise exactly-once.
* `LinkStats` returns counts and no PII (VYQ-22).

## Considered Options

* In-process batcher, count increments, best-effort delivery
* Synchronous insert on the redirect path
* An external analytics collector with raw request storage
* Exactly-once delivery via an outbox and a broker

## Decision Outcome

Chosen option: "in-process batcher, count increments, best-effort delivery".

What a click is:

* An increment associated with a public code (and therefore a link), not
  a stored HTTP request.
* No client IP, user agent, referrer, cookie, token, or destination URL
  in the click row or in the log line for that event.
* Stats queries return counts. They do not return a visit list.

How it is recorded:

* Resolve does not write the click. The redirect handler invokes
  `RecordClick` and does not wait for the store (VYQ-22, VYQ-34).
* `RecordClick` hands the event to an async port.
* The batcher (VYQ-28) uses a bounded channel, flushes every N events or
  every T seconds, and drains on shutdown.
* If the channel is full, the click is dropped and a metric counts the
  drop. The redirect still returns 302.
* `Server.Shutdown` must drain the batcher (VYQ-38). A SIGTERM that skips
  the flush is a bug.
* If the process dies before flush, those clicks are gone. Document that
  in the operations README (VYQ-42). Do not retry from a client hint, and
  do not claim at-least-once or exactly-once.

Decorators and access logs follow the same privacy rule
([ADR-0002](0002-clean-architecture-and-wild-workouts.md)). A metric may
count requests, status classes, and dropped clicks. It must not label
those series with a destination URL, a raw IP, or a token.

The resolve LRU (VYQ-29) caches the read model for redirect. It is not
an analytics store. It stores no request metadata, and delete invalidates it.

### Consequences

* Good, because redirect latency is the resolve path, not the click insert.
* Good, because a database backup of click rows is not a browsing history.
* Bad, because counters are approximate. Operators must not bill or audit
  against them as if they were complete.
* Bad, because a full channel silently loses clicks aside from the drop
  metric. That is preferred to blocking the redirect or unbounded memory.
* Neutral, because SQLite's single writer
  ([ADR-0005](0005-postgres-and-sqlite-via-database-sql.md)) is another
  reason to batch. Batching does not remove that limit.

### Confirmation

* VYQ-22: `RecordClick` does not wait for the click repository to finish.
* VYQ-28: batcher tests pass, including with `testing/synctest`; a
  shutdown with pending events persists them; overflow drops and counts.
* VYQ-38: a SIGTERM in test does not skip the batcher flush.
* VYQ-42: the operations README states that analytics are approximate.

## Pros and Cons of the Options

### In-process batcher and counts

* Good, because it needs no broker and no new `require`.
* Good, because privacy is structural: the data is never accepted.
* Bad, because durability is limited to what has been flushed.

### Synchronous insert

* Good, because a returned 302 would mean the click was stored, when the
  insert succeeded.
* Bad, because the redirect waits on the database, and a slow or locked
  SQLite writer delays every visit. Rejected.

### External collector with raw requests

* Good, because a dedicated system can store more dimensions.
* Bad, because it is the opposite of the privacy rule, and it is another
  service this self-hosted binary does not include.

### Exactly-once outbox and broker

* Good, because crash loss would shrink.
* Bad, because it needs a broker or a two-phase protocol, and
  `AGENTS.md` forbids promising exactly-once. Rejected.

## More Information

* Redirect status is independent of click success:
  [ADR-0007](0007-temporary-302-redirects.md).
* Rate limits may count requests in memory. Those counters are not click
  analytics and must not be written as visit logs:
  [ADR-0010](0010-abuse-prevention.md).
