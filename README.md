# Veylqo — Less URL. More destination.

Self-hosted, open-source link shortener and share service. Go 1.27,
standard library first. The JS SDK lives in
[`jovrion/veylqo-js`](https://github.com/jovrion/veylqo-js).

Public shortening is not an open redirect. `VEYLQO_ALLOWED_HOSTS` is
required. An empty list fails closed. Only `http` and `https`
destinations are accepted. Private and loopback hosts are blocked even
if they are listed. The publishable key and the admin token must be
distinct.

## Clone

```
git clone https://github.com/jovrion/veylqo.git
cd veylqo
```

The Go module is `github.com/jovrion/veylqo` (`go 1.27`). Board commands
need only Python 3 from the standard library.

```
make fmt     # gofmt -s -w .
make vet     # go vet ./...
make test    # go test ./...
make build   # go build ./...
```

## Run the binary

Build and start the process. Credentials are environment variables so
they do not appear in the process argument list. Replace the two keys
before any shared or public host. Do not commit a `.env` file.

```
go build -o veylqo ./cmd/veylqo

export VEYLQO_DB_URL='sqlite://file:veylqo.db?mode=rwc'
export VEYLQO_ALLOWED_HOSTS='example.com'
export VEYLQO_PUBLISHABLE_KEY='replace-me-publishable'
export VEYLQO_ADMIN_TOKEN='replace-me-admin'
./veylqo
```

The process listens on `:8080` unless `-addr` or `VEYLQO_ADDR` says
otherwise. A flag overrides the environment for `-addr` and `-migrate`.
Migrations run on start. Pass `-migrate=false` or set
`VEYLQO_MIGRATE=false` to skip them.

`GET /healthz` and `GET /readyz` are the probes. `GET /metrics` counts
handler calls by status class and dropped clicks. It does not label a
series with a destination URL, a raw IP, or a token.

`SIGINT` or `SIGTERM` drains the click batcher, then shuts the listener
down. The default shutdown budget is 15s (`VEYLQO_SHUTDOWN_TIMEOUT`).
A crash before that flush loses clicks that were still queued.

## Docker Compose

`docker-compose.yml` runs Postgres 16 and the image from the
`Dockerfile` (`CGO_ENABLED=0`, distroless, no shell). The values in that
file are placeholders for a private machine. Override the keys and
`VEYLQO_ALLOWED_HOSTS` in the environment. Do not commit a `.env`.

```
export VEYLQO_ALLOWED_HOSTS='example.com'
export VEYLQO_PUBLISHABLE_KEY='replace-me-publishable'
export VEYLQO_ADMIN_TOKEN='replace-me-admin'
docker compose up --build
```

The service publishes `8080`. Probe from outside the container:
`GET /healthz` and `GET /readyz`. The image has no shell.
`stop_grace_period` is 20s so a stop can finish the default 15s drain.

`VEYLQO_DB_URL` in Compose uses the `postgres://` scheme, not
`postgresql://`. `sslmode=disable` matches the Compose Postgres image,
which does not terminate TLS. Use TLS to a real Postgres host.

## Environment

Comma-separated lists split on `,` and ignore empty fields. The process
does not log `VEYLQO_DB_URL`, keys, or tokens. Error text does not echo
them.

| Variable | Required | Meaning |
| --- | --- | --- |
| `VEYLQO_DB_URL` | yes | `postgres://` or `sqlite://`. Other schemes are rejected. |
| `VEYLQO_ALLOWED_HOSTS` | yes | Destination hosts that may be shortened. ASCII or punycode. Empty fails closed. |
| `VEYLQO_PUBLISHABLE_KEY` | yes | Authorizes `POST /api/v1/links` only. Safe to embed in a page. Not an admin credential. |
| `VEYLQO_ADMIN_TOKEN` | yes | Authorizes get, delete, and stats. Must differ from the publishable key. |
| `VEYLQO_ADDR` | no | Listen address. Default `:8080`. Overridden by `-addr`. |
| `VEYLQO_MIGRATE` | no | Apply embedded migrations on start. Default true. Overridden by `-migrate` when that flag is passed. |
| `VEYLQO_CORS_ORIGINS` | no | Browser origins allowed to call the API. Unset means none. |
| `VEYLQO_TRUSTED_PROXIES` | no | Comma-separated IPs or CIDRs. Empty trusts no proxy. |
| `VEYLQO_RATE_BURST` | no | Per-IP token-bucket size. Default 60. |
| `VEYLQO_RATE_REFILL` | no | Go duration between tokens. Default `1s`. |
| `VEYLQO_CLICK_CAPACITY` | no | Click queue size. `0` or unset uses 1024. Negative is rejected. |
| `VEYLQO_CLICK_BATCH` | no | Flush size. `0` or unset uses 64. |
| `VEYLQO_CLICK_INTERVAL` | no | Flush interval. `0` or unset uses `1s`. |
| `VEYLQO_SHUTDOWN_TIMEOUT` | no | Drain budget after `SIGTERM`. Default `15s`. Must be positive. |
| `VEYLQO_ADDR_FILE` | no | If set, the bound address is written there. Used by tests. |

`VEYLQO_BASE_URL` is not read by the process. Setting it has no effect.
Short links are served by this listener. A short domain in front of it
is a reverse-proxy concern, not an application setting.

`VEYLQO_TEST_PG_DSN` is a test-only Postgres DSN. The server does not
read it. Do not point production at a test database, and do not print
the DSN.

Allowed hosts are compared as ASCII or punycode. The standard library
does not normalize IDN. Convert Unicode labels before listing them.
An IP literal is allowed only if that address is on the list and is not
private, loopback, link-local, or unspecified. The block wins over the
list.

The rate limit is per process. Replicas do not share a bucket. Excess
is 429. The limiter is not a visit log.

## Short domain

Put TLS and the public hostname on a reverse proxy. Forward to the
listener (`:8080` by default). Do not rewrite the redirect to 301.
`GET /{code}` returns **302**, with `Cache-Control: private, max-age=0`.
The handler sends the stored destination in `Location`. It does not
connect to that host.

The client IP used for the rate limit is the connection address, unless
the peer is in `VEYLQO_TRUSTED_PROXIES`. Only then may a forwarded
header be used. An untrusted `X-Forwarded-For` is ignored. Set the
variable to the proxy's IP or CIDR, not to the public internet. An
invalid entry fails startup and is not echoed.

`GET /{code}` needs no publishable key. It only redirects to a
destination that passed the allowlist when the link was created. DNS
changes after that are not re-checked on each visit.

## SQLite

SQLite is the small-install engine. It has one writer. It does not
replace Postgres under concurrent writes or under load. A click flush
and a shorten can contend on that writer. The batcher keeps redirect
latency off that lock. It does not make SQLite a multi-writer database.

Use a file URL the process can create and write, for example
`sqlite://file:veylqo.db?mode=rwc`. Do not put that file on a network
share and point several processes at it. For more than one writer, use
Postgres (`postgres://`).

## Analytics

Click analytics are counts for a public code, not a stored HTTP
request. No client IP, user agent, referrer, cookie, token, or
destination URL is written on the click row.

Counts are approximate. `RecordClick` does not block the redirect. If
the queue is full, the click is dropped, a metric increments, and the
response is still 302. Shutdown drains the queue. If the process dies
before flush, those clicks are gone. Do not promise at-least-once or
exactly-once. Do not bill or audit from these counters.

## Branches

`main` is the default branch. The `protect-main` ruleset
(`.github/rulesets/protect-main.json`) rejects a direct push, a force
push, and deletion. Land changes from a branch with a pull request.
Required checks are `test` and `postgres`. No approving review is
required, so one maintainer can merge after CI is green. Use squash or
rebase: merge commits are rejected.

The same ruleset, with check `test` only, lives in `jovrion/veylqo-js`.

## How work is tracked

Product changes land through the board in `work/`. A task describes
intent until its status is `done`.

```
make work-next              # next ready task (phase, then priority)
make work-start ID=VYQ-10   # claim: open → in-progress (WIP = 1)
make work-done  ID=VYQ-10   # close when every Verification box is ticked
make work-validate          # schema, cycles, WIP, fresh BOARD.md
make work-view              # Kanban + map (NOOPEN=1 to only generate)
```

Rules: `WORKFLOW.md`, `work/README.md`, `AGENTS.md`.
OpenCode: `/work-next`, `/work-start`, `/work-done`.

## License

MIT. See `LICENSE`.
