---
id: VYQ-16
type: task
title: "[go] Domain Cleaner service"
status: "done"
priority: "major"
area: "go"
epic: VYQ-2
depends_on: [VYQ-15]
blocked_reason: null
updated: 2026-11-10
---

# VYQ-16 — [go] Domain Cleaner service

## Scope

Strip `utm_*`, `fbclid`, `gclid`, `ref`, `tag`, and similar. Optional per-host rules (e.g. keep `/dp/ASIN`). `clean: false` option. Table tests. ADR-0008 once it exists.

## Verification

Tick only with re-runnable evidence. `make work-done` refuses to close if any box is open.

- [x] `go test ./internal/links/domain/link/ -run Clean` green
- [x] a URL with `utm_source` comes out without that query

## Evidence & Notes

2026-11-10:

```
gofmt -s -w .
go vet ./...
go test ./internal/links/domain/link/ -run Clean -count=1
ok  github.com/jovrion/veylqo/internal/links/domain/link
go test ./internal/archtest/... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
go test ./... -count=1
ok  github.com/jovrion/veylqo/internal/archtest
ok  github.com/jovrion/veylqo/internal/links/domain/link
```

`TestClean/utm_source_stripped` takes `https://example.com/a?utm_source=campaign&b=1` and expects `https://example.com/a?b=1`. The failure text does not print the query.

`Clean` uses `net/url` only. It strips `utm_*` (prefix, case-insensitive), `fbclid`, `gclid`, `ref`, `tag`, and the same class of click ids (`mc_eid`, `igshid`, `msclkid`, `gbraid`, `wbraid`, `twclid`, `ttclid`, `dclid`, `yclid`). `Skip` is `clean: false`: it keeps those parameters and skips per-host path rules. It still drops the fragment and still rejects `javascript:` and loopback. No fetch, no IDN conversion, no new dependency.

The default host rule keeps `/dp/<ASIN>` on Amazon storefronts only. A custom `HostRule` can name another host. `NewLink` does not call `Clean`; the shorten command does that (VYQ-20). The allowlist stays there too.
