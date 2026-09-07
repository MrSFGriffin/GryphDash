# GryphDash

A personal dashboard for live Codex usage and limits, with clearly labeled
OpenRouter demo data. The Go server embeds its HTML and CSS in a single executable
and uses only the standard library. Live Codex metrics additionally require the
Codex CLI on the same machine.

## Run

Install Go 1.22 or later and the Codex CLI. Authenticate with your ChatGPT account
as the OS user who will run the dashboard:

```sh
codex login
codex login status
go run .
```

An existing ChatGPT login can be reused. No API key or token environment variable
is needed. Codex manages its own credentials; do not copy `auth.json` into this
repository. An API-key-only login does not provide these subscription metrics.

Open <http://127.0.0.1:8080>. Stop with Ctrl+C. The first read runs in the background;
the page refreshes every minute. Only one polling cycle runs at a time, with a
45-second timeout. Each cycle starts a local `codex app-server` over stdin/stdout,
reads metrics, and stops it. No model tasks are started or earned resets redeemed.

## Metrics

- Account plan and authentication type.
- All returned limit buckets, including five-hour and weekly usage percentages,
  remaining percentages, reset timestamps, and countdowns at page render time.
- Credit balance, credit availability, and unlimited-credit status.
- Per-bucket plan, label, reached-limit state, spend-control status, and individual
  spend limits when supplied (including used amount, allowance, remaining
  percentage, and reset time).
- Earned reset count and every returned reset's title, description, status, type,
  identifier, grant time, and expiration time. These are distinct from credits
  available to spend. The provider may return fewer detail rows than the count.
- Lifetime tokens, peak daily tokens, longest-running turn in seconds, current
  streak, and longest streak in days.
- Every returned daily token total, with relative activity bars. Missing dates
  are not assumed to have zero usage.

Account, limits, and activity refresh independently within each polling cycle.
Successful sections remain usable when another fails. Failed reads preserve the
last successful data and show a stale status; missing fields say `Unavailable`.
A zero balance or streak is displayed as zero. Times are UTC. Codex data is kept
in memory, so restarting clears the cache. OpenRouter is not connected yet.

The adapter uses the documented [`account/read`, `account/rateLimits/read`, and
`account/usage/read`](https://learn.chatgpt.com/docs/app-server) methods. It was
verified against Codex CLI 0.153.4. Field and method availability depends on CLI
version and account. Account-wide activity is shown as reported by Codex; the
server does not scrape conversations or request per-thread billing estimates.

## Configuration

| Environment variable | Default | Meaning |
| --- | --- | --- |
| `GRYPHDASH_ADDR` | `127.0.0.1:8080` | HTTP listen address |
| `GRYPHDASH_CODEX_BIN` | `codex` | CLI executable name or full path (not shell arguments) |
| `GRYPHDASH_REFRESH_INTERVAL` | `1m` | Delay after each polling cycle; minimum `30s` |

```sh
GRYPHDASH_ADDR=127.0.0.1:9090 GRYPHDASH_REFRESH_INTERVAL=2m go run .
```

The server and its Codex child inherit the shell environment, including an
existing `CODEX_HOME` override. `.env` is gitignored but is not automatically
loaded. Use shell environment variables or your service manager's configuration.

The default listener is local only. There is no authentication: binding to
`0.0.0.0:8080` exposes real account metrics through other network interfaces.

## Build and check

```sh
go test ./...
go vet ./...
go build -o bin/gryphdash .
./bin/gryphdash
```

With a C compiler installed, also run `CGO_ENABLED=1 go test -race ./...`.

The binary runs from any directory without templates or other source files.
The Codex CLI and its login must still be available. Rebuild after changing the
embedded page in `web/dashboard.html`.

See [PLAN.md](PLAN.md) for the remaining work.
