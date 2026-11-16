---
id: VYQ-51
type: epic
title: F7 — Private attribution and declarative SDK
phase: F7
depends_on: [VYQ-6, VYQ-7]
updated: 2026-11-15
---

# VYQ-51 — F7 — Private attribution and declarative SDK

## Goal

Turn the private backend into the place that mints a short link on a
Veylqo-owned host, shaped `https://<public-base>/{prefix}/{code}`, and
that records share and click events per customer site. The JS SDK stays
framework-free: a page loads the script, sets `<meta name="veylqo:*">`,
and marks a control with `data-veylqo-link`. A click copies that short
URL. It does not use the customer origin as the short-link host.

`prefix` is a short distinctive token for the destination site (derived
from its host, or overridden). `code` is the public code. The backend
repo is private because those events are customer data. `veylqo-js`
stays the distributable SDK.

## Done criteria

A page that loads the CDN bundle and declares
`<meta name="veylqo:endpoint">`, `<meta name="veylqo:key">`, and
`<a data-veylqo-link="true">` copies a short URL of the form
`https://<public-base>/{prefix}/{code}`. Opening that URL returns 302
to the stored destination and records a click for that site. A second
copy of a cached code still records a share. Stats stay approximate
counts, with share/click breakdowns and no raw IP, user agent, token,
or destination query. ADR-0012, ADR-0013, and ADR-0014 are accepted.
The backend repository is private.
