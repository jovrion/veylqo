# Security

Veylqo is a self-hosted shortener. You run the process and the database.
There is no hosted tenancy and no vendor key escrow.

## Reporting

Do not open a public issue that contains a destination URL with a query
string, a client IP, a publishable key, an admin token, or a database
URL. Those values are credentials or private request data.

Report a vulnerability through GitHub private security advisories on
[jovrion/veylqo](https://github.com/jovrion/veylqo/security/advisories).
Include the version or commit, what you expected, and what happened.
Omit live secrets. Rotate any key or token that may have leaked before
you write the report.

## What the process enforces

Public shortening is not an open redirect.

- `VEYLQO_ALLOWED_HOSTS` is required. An empty list fails closed.
- Destinations must be `http` or `https`. Other schemes are rejected.
- Private, loopback, link-local, and unspecified addresses are blocked
  even if someone adds them to the allowlist.
- Hosts are ASCII or punycode. The standard library does not normalize
  IDN.
- `VEYLQO_PUBLISHABLE_KEY` authorizes `POST /api/v1/links` only.
- `VEYLQO_ADMIN_TOKEN` authorizes get, delete, and stats. It must be
  distinct from the publishable key. Knowing a public code is not admin
  access.
- `GET /{code}` needs no publishable key. It emits the stored
  destination. It does not connect to that host. The status is 302,
  never 301.
- The rate limit is a per-IP token bucket in this process. The client
  IP is the connection address unless the peer is listed in
  `VEYLQO_TRUSTED_PROXIES`. Untrusted forwarded headers are ignored.
  Replicas do not share a bucket.
- The process does not log destination URLs with query strings, client
  IPs, or tokens. Click rows are counts, not request logs.

DNS changes after a link is created are not re-checked on every
redirect. The redirect still only sends the stored URL.

## Operator duties

- Replace Compose placeholders before any shared or public host.
- Do not commit a `.env`, a database file, or a real key.
- Terminate TLS on the reverse proxy. Set `VEYLQO_TRUSTED_PROXIES` to
  that proxy only.
- Prefer Postgres when more than one writer is required. SQLite is one
  writer and is not a stand-in for Postgres under load.
- Treat click counts as approximate. A crash before flush loses queued
  clicks.
