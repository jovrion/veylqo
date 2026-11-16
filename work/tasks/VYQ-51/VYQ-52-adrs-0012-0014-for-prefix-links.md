---
id: VYQ-52
type: task
title: "[docs] ADRs 0012-0014 for prefix links, attribution, and the declarative SDK"
status: "open"
priority: "critical"
area: "docs"
epic: VYQ-51
depends_on: []
blocked_reason: null
updated: 2026-11-15
---

# VYQ-52 — [docs] ADRs 0012-0014 for prefix links, attribution, and the declarative SDK

## Scope

Accepted MADR decisions, then a short pointer in `docs/architecture.md`.
Do not rewrite ADR-0001 through ADR-0011. Each new ADR says which
decision it amends. No product code. No new `require`.

ADR-0012 (amends ADR-0004, ADR-0006, ADR-0007):

- New random codes are 8-character base62. Existing 3–64 codes stay valid.
- The public short URL is `https://<VEYLQO_PUBLIC_BASE_URL>/{prefix}/{code}`,
  not the customer origin and not `/{code}` alone. `GET /{code}` remains
  for links that already exist.
- `GET /{prefix}/{code}` is 302, never 301, with
  `Cache-Control: private, max-age=0`. A prefix that does not match the
  stored prefix is 404. Expired or deleted is 410 and is not redirected.
- `prefix` is 1–16 characters matching `[A-Za-z0-9_-]+`. Reserved names
  (`api`, `healthz`, `readyz`, `metrics`, `debug`, `static`), including
  case variants, are rejected. The fallback prefix is `d`.
- When the caller omits `prefix`, derive it from the destination host.
  The server is the authority. The algorithm is data in the domain, not
  a public-suffix library: lowercase; drop a leading label while it is
  `www`, `m`, `app`, `web`, `shop`, or `store`; strip the longest
  matching suffix from a built-in list (`co.uk`, `com.co`, `edu.co`,
  `com.au`, `co.jp`, `com`, `net`, `org`, `io`, `co`, `es`, and the same
  class); abbreviate the brand label. A label of 2–4 characters is kept.
  A hyphenated label uses the initial of each part, up to 4. A longer
  label is the first letter plus the next distinct consonants until 3
  characters, or the first 3 letters if there are not enough consonants.
  Document this table in the ADR: `unisimon.edu.co` → `uns`,
  `mi-tienda-online.com` → `mto`, `nike.com` → `nike`,
  `amazon.com` → `amz`, `www.bbva.es` → `bbva`. A reserved result falls
  back to `d`. An explicit prefix override wins when it validates.
- `POST /api/v1/links` returns `code`, `prefix`, and `short_url`.
  `short_url` has no query string.

ADR-0013 (supersedes the "counts only, no event rows" limit in ADR-0009;
amends ADR-0005 and ADR-0010). Privacy limits in ADR-0009 still hold:

- The backend repository is private. Share and click events are customer
  data. Analytics remain approximate. Do not promise exactly-once.
- A site is selected by its publishable key. The key is stored as a hash.
  The existing `VEYLQO_PUBLISHABLE_KEY` plus `VEYLQO_ALLOWED_HOSTS` is
  the default site so a current install keeps working. Further sites are
  created with the admin token (`POST /api/v1/sites`) and listed without
  secrets (`GET /api/v1/sites`). The raw key is returned once.
- A link and its events belong to that site. Idempotent reuse does not
  cross sites. A destination host must be on that site's allowlist.
  Empty allowlist fails closed. Private and loopback hosts stay blocked.
- Record a `share` when a link is created, and when the SDK reports
  another copy of a cached code (`POST /api/v1/links/{code}/shares`).
  Record a `click` on a successful 302 only. Do not store a raw IP, raw
  User-Agent, token, full referrer, or destination query. A click may
  store `referrer_host` and a device class
  (`mobile`, `desktop`, `tablet`, `bot`, `unknown`). A share may store
  the page host the SDK sends and `channel=copy`.
- Stats stay admin-only. The publishable key cannot read them. The stats
  payload gains shares, clicks, and breakdowns. It is still not a visit list.

ADR-0014 (amends ADR-0011):

- The SDK remains vanilla. No UI runtime. The admin token must not appear
  in SDK examples.
- Global config is `<meta name="veylqo:endpoint">`,
  `<meta name="veylqo:key">`, optional `<meta name="veylqo:prefix">`,
  and `<meta name="veylqo:mode">` (`prod` default, or `debug`).
- A control is any element with `data-veylqo-link="true"`. Optional
  overrides: `data-veylqo-url`, `data-veylqo-prefix`, `data-veylqo-slug`,
  `data-veylqo-clean`. The SDK sets `data-veylqo-state`.
- The action is always copy of `short_url`. Do not open the Web Share
  sheet on this path. `<veylqo-share>` stays.
- `prod` logs no success and, on failure, only a status and slug.
  `debug` may log a redacted diagnostic. Neither mode logs the
  publishable key or a destination query string.
- The short host is the server `short_url`. The SDK must not build the
  public link from `window.location.origin`.

`docs/architecture.md` links these three ADRs from the shorten flow,
the resolve flow, and the out-of-MVP list. It does not copy the decision
text. Remove or reclassify only the deferred items these ADRs explicitly
accept (event rows for share/click, a brand prefix on the public path).
301, open redirect, raw request logs, and a third-party public-suffix
library stay out.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [ ] `docs/adr/0012-*.md`, `docs/adr/0013-*.md`, and `docs/adr/0014-*.md` each contain the line `Status: Accepted`
- [ ] `docs/architecture.md` links ADR-0012, ADR-0013, and ADR-0014
- [ ] `make work-validate` exits 0

## Evidence & Notes
