# GryphDash

A customizable widget dashboard for metrics from multiple providers. The Go server
uses only the standard library and embeds all HTML, CSS, and JavaScript, including
GridStack, in a single executable. Live metrics require the Codex CLI on the same
machine. Widget definitions live in the embedded [`widgets.json`](widgets.json)
catalog, so adding a metric does not require editing the dashboard renderer.
OpenRouter widgets use the optional `OPENROUTER_API_KEY` when it is configured.

## Run

Install Go 1.24 or later and the Codex CLI. Authenticate with your ChatGPT account
as the OS user who will run the dashboard:

```sh
codex login
codex login status
go run . web
```

An existing ChatGPT login can be reused. No API key or token environment variable
is needed. Codex manages its own credentials; do not copy `auth.json` into this
repository. An API-key-only login does not provide these subscription metrics.

Open <http://127.0.0.1:8080>. Stop with Ctrl+C. The first read runs in the background;
widget values refresh in place every 15 seconds from the server cache. Provider
reads run about once per minute by default. Only one polling cycle runs at a time, with a
45-second timeout. Each cycle starts a local `codex app-server` over stdin/stdout,
reads metrics, and stops it. No model tasks are started or earned resets redeemed.

To use the terminal dashboard instead, run `go run . tui` (or `./bin/gryphdash tui`
after building). It uses the same provider collectors and widget catalog, refreshes
on the configured interval, and runs in a full-screen Bubble Tea interface styled
with Lip Gloss. Press `a` to open the widget picker; use `/` to search with
order-independent terms across widget names, groups, and descriptions. Use
left/right or Tab to move between the All tab and provider-group tabs, and arrows to focus widgets. Use
Shift+arrow or Shift+`h/j/k/l` to reorder widgets, `d` to remove the focused widget, `r` to restore defaults,
`n` to name the current layout, and `m` to manage saved layouts. Press `q`, `Esc`,
or Ctrl+C to quit. The selection, order, and named layouts are saved in
`~/.config/gryphdash/layout.json` (or the platform equivalent). `web` is the
default mode when no command is supplied.

## Customize your dashboard

- **Add widgets** opens a searchable picker of every available provider metric.
- **Edit dashboard** enables dragging by a widget heading, corner resizing, and
  removal with the × button. Removed widgets remain available in the picker.
- For keyboard editing, Tab to a widget heading and use arrow keys to move it;
  Shift + arrow keys changes its size. **Done editing** locks the arrangement.
- **Default**, available from the layout selector, restores the initial selection.
- Layout and widget selection save automatically in this browser's `localStorage`
  under `gryphdash.layout.v1` and named layouts under `gryphdash.layouts.v1`, including OpenRouter widgets. The current layout saves automatically; **Create named layout** creates or updates a named snapshot that can be loaded from the dashboard controls. They survive page reloads and server restarts, and
  an intentionally empty dashboard stays empty. Storage contains only widget IDs
  and positions/sizes. It does not store metrics or credentials.

Layouts are specific to the browser and origin (including port); they do not sync
between devices. If storage is blocked or corrupt, the dashboard remains usable
and displays a message. The grid adapts to mobile screens without overwriting the
saved desktop arrangement. A saved widget whose source temporarily disappears
stays in place and shows an unavailable state.

Limits, balances, activity summaries, and account fields are individual widgets.
Daily activity and earned-reset details have larger dedicated widgets. A small
initial selection is shown; all other metrics can be added from the picker.

## Widget catalog

`widgets.json` is the source of truth for the widget picker. Each definition has a
stable `id`, `group`, display `name`, user-facing `description`, default layout
size/visibility, and `logic`. Logic names are interpreted by the server: `scalar`
reads a dotted field from an account, limits, or usage response; `limitWindow`
adds remaining percentage and reset handling for a limit bucket; `daily` and
`resetDetails` render structured lists; `timestamp` formats Unix timestamps; and
`url` metadata records the HTTP source for provider adapters. Definitions
with `scope: "limitBuckets"` expand once for every provider-returned bucket.

The server validates required catalog fields at startup and embeds the JSON in the
binary. Keep IDs stable when changing names or descriptions so users' saved
browser layouts continue to work.

## Add custom widgets

Widget definitions are data-driven. To add a metric from a provider that is
already supported, add an entry to [`widgets.json`](widgets.json); do not edit
the web or TUI renderers. Each entry needs a stable `id`, `group`, `name`,
`description`, `width`, `height`, and `logic` object. For example:

```json
{
  "id": "openrouter/spend/monthly",
  "group": "OpenRouter",
  "name": "Monthly spend",
  "description": "Usage charged this month · USD",
  "default": false,
  "width": 4,
  "height": 4,
  "logic": { "type": "scalar", "source": "openrouterKey", "path": "usage_monthly" }
}
```

Use `scalar` for a single field, `timestamp` for a date, `daily` for token
activity, `resetDetails` for reset lists, and `limitWindow` for a limit bucket.
The `source` and `path` must match data returned by the provider adapter. Keep
IDs stable so saved browser and TUI layouts continue to work.

### Adding a new provider

For a new external service, such as a stock-price, music, or calendar service:

1. Create a package directory such as `providers/<name>/`.
2. Add a client that reads credentials from environment variables, calls the
   service, and returns JSON-like fields in named result sections. Keep all HTTP
   and response parsing in this package.
3. Add `providers/<name>/client_test.go` using an `httptest` fixture. Cover
   successful responses, authentication failures, malformed responses, and any
   provider-specific limits. Tests must never call the live service.
4. Add a provider reader in `collector.go` that adapts the client result to named
   sections, then register it in the collector's provider list. A client section
   named `metric` might be exposed as source `<name>` with path `metric`.
5. Add widget definitions to `widgets.json`:

```json
{
  "id": "example/item/metric",
  "group": "Example provider",
  "name": "Metric value",
  "description": "A value returned by the provider",
  "default": false,
  "width": 4,
  "height": 4,
  "logic": { "type": "scalar", "source": "example", "path": "value" }
}
```

6. Add required environment variables to the Configuration table and explain
   how to obtain them. Never put credentials in `widgets.json`, source code, or
   browser storage.
7. Run the standard Go checks. The web app and TUI discover the new widget from
   the catalog; renderer changes are not needed.

Provider API clients and their tests belong under `providers/<name>/`; collector
registration is the only application-level wiring currently required.

## Metrics

- Account plan and authentication type.
- All returned limit buckets, including five-hour and weekly usage percentages,
  remaining percentages, reset timestamps, and ticking reset countdowns.
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
in memory, so restarting clears the metric cache but not the browser layout.

The adapter uses the documented [`account/read`, `account/rateLimits/read`, and
`account/usage/read`](https://learn.chatgpt.com/docs/app-server) methods. It was
verified against Codex CLI 0.153.4. Field and method availability depends on CLI
version and account. Account-wide activity is shown as reported by Codex; the
server does not scrape conversations or request per-thread billing estimates.

OpenRouter uses `GET /api/v1/key` for current API-key usage, monthly usage,
configured limit, remaining limit, reset period, and expiration. It also attempts
`GET /api/v1/credits` for purchased and used credits. OpenRouter may require a
management key for the credits endpoint; a 401/403 is shown on that widget rather
than treated as a zero balance. See the [OpenRouter current-key documentation](https://openrouter.ai/docs/api/api-reference/api-keys/get-current-key)
and [credits documentation](https://openrouter.ai/docs/api/api-reference/credits/get-credits).

## Configuration

| Environment variable | Default | Meaning |
| --- | --- | --- |
| `GRYPHDASH_ADDR` | `127.0.0.1:8080` | HTTP listen address |
| `GRYPHDASH_CODEX_BIN` | `codex` | CLI executable name or full path (not shell arguments) |
| `GRYPHDASH_REFRESH_INTERVAL` | `1m` | Delay after each polling cycle; minimum `30s` |
| `OPENROUTER_API_KEY` | unset | Bearer key for OpenRouter key usage and credits requests |

```sh
GRYPHDASH_ADDR=127.0.0.1:9090 GRYPHDASH_REFRESH_INTERVAL=2m go run .
```

The server and its Codex child inherit the shell environment, including an
existing `CODEX_HOME` override. `.env` is gitignored but is not automatically
loaded. Use shell environment variables or your service manager's configuration.
For example: `OPENROUTER_API_KEY=sk-or-v1-... go run .`. The key is kept in
memory and is never sent to the browser or written to logs. Without the variable,
OpenRouter widgets display an unavailable configuration status.

The default listener is local only. There is no authentication: binding to
`0.0.0.0:8080` exposes real account metrics through other network interfaces.

## Build and check

```sh
go test ./...
go vet ./...
go build -o bin/gryphdash .
./bin/gryphdash
./bin/gryphdash tui
```

With a C compiler installed, also run `CGO_ENABLED=1 go test -race ./...`.

The binary runs from any directory without templates or other source files.
The Codex CLI and its login must still be available. Rebuild after changing the
embedded assets in `web/`. GridStack 13.2.0 is vendored with its MIT license in
`web/vendor/gridstack/`; no CDN access or Node.js installation is needed to run
or build the server. `/api/widgets` serves the cached widget catalog and values;
visiting it does not trigger a provider read.

## Browser tests (optional development tools)

`tests/browser.mjs` exercises adding/removing widgets, pointer dragging/resizing,
keyboard resizing, reload persistence, live updates, mobile layout, empty layouts,
and storage failure recovery using fictional metrics. It starts its own server
and does not use your Codex login. Build the binary first.

Install Playwright and Chromium in a temporary directory, then run:

```sh
npm install --prefix /tmp/gryphdash-browser-tests --no-audit --no-fund playwright@1.58.2
PLAYWRIGHT_BROWSERS_PATH=/tmp/gryphdash-browser-tests/browsers /tmp/gryphdash-browser-tests/node_modules/.bin/playwright install chromium
GRYPHDASH_PLAYWRIGHT_MODULE=/tmp/gryphdash-browser-tests/node_modules/playwright/index.mjs PLAYWRIGHT_BROWSERS_PATH=/tmp/gryphdash-browser-tests/browsers node tests/browser.mjs
```

The host needs Chromium's system libraries. On Ubuntu 26.04, Playwright 1.58.2
needs `PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu24.04-x64` during browser installation
and test execution to use its fallback build. Tests use port 18091 by default
(override with `GRYPHDASH_TEST_PORT`). Screenshots are written under `/tmp`.
