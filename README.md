<h1><img src="web/logo.svg" alt="GryphDash logo" width="32" height="32"> GryphDash</h1>

A customizable widget dashboard for metrics from multiple providers. The Go server
uses only the standard library and embeds all HTML, CSS, and JavaScript, including
GridStack, in a single executable. Live metrics require the Codex CLI on the same
machine. Built-in widget definitions live in the embedded [`widgets.json`](internal/dashboard/widgets.json)
catalog, while external subprocess providers supply their own widget definitions at runtime.
OpenRouter widgets use the optional `OPENROUTER_API_KEY` when it is configured.

## Run

Install Go 1.24 or later and the Codex CLI. Authenticate with your ChatGPT account
as the OS user who will run the dashboard:

```sh
codex login
codex login status
go run . web
```

For local development, `./run-web.sh` builds the subprocess providers and
starts the web dashboard with them configured automatically.

An existing ChatGPT login can be reused. No API key or token environment variable
is needed. Codex manages its own credentials; do not copy `auth.json` into this
repository. An API-key-only login does not provide these subscription metrics.

Open <http://127.0.0.1:8080>. Stop with Ctrl+C. The first read runs in the background;
widget values refresh in place every 15 seconds from the server cache. Provider
reads run about once per minute by default. Only one polling cycle runs at a time, with a
45-second timeout. Each cycle starts a local `codex app-server` over stdin/stdout,
reads metrics, and stops it. No model tasks are started or earned resets redeemed.

To use the terminal dashboard instead, run `./run-tui.sh` or `go run . tui` (or `./bin/gryphdash tui`
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

[`widgets.json`](internal/dashboard/widgets.json) is the source of truth for built-in widgets in the widget picker. Each definition has a
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
already supported, add an entry to [`widgets.json`](internal/dashboard/widgets.json); do not edit
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
  "logic": { "type": "scalar", "source": "openrouter/key", "path": "usage_monthly" }
}
```

Use `scalar` for a single field, `timestamp` for a date, `daily` for token
activity, `resetDetails` for reset lists, and `limitWindow` for a limit bucket.
The `source` and `path` must match data returned by the provider adapter. Keep
IDs stable so saved browser and TUI layouts continue to work.

### Adding a new provider

For a new external service, such as a stock-price, music, or calendar service:

1. Create a package directory such as `providers/<name>/`.
2. Add a provider executable under `cmd/` that supports `-widgets` and the
   line-delimited `read` protocol. The executable owns credentials, API calls,
   response parsing, source naming, and widget definitions.
3. Add provider tests using an `httptest` fixture. Cover
   successful responses, authentication failures, malformed responses, and any
   provider-specific limits. Tests must never call the live service.
4. Build the executable as `gryphdash-provider-<name>` and put it beside
   GryphDash or in the directory named by `GRYPHDASH_PROVIDER_DIR`.
5. Document required environment variables and run the standard Go checks.

Provider API clients, normalization, widget definitions, and their tests belong
under `providers/<name>/`. Application wiring only discovers generic provider
executables and passes their results to the collector. Do not add
provider-specific fields, imports, response parsing, or source switches to
`internal/collector`.

### External subprocess providers

External providers use a line-delimited JSON protocol on standard input and
output. The host runs the executable with `-widgets` at startup and expects one
response containing the provider name and its complete widget catalog. It then
sends `{"version":1,"method":"read"}` for each refresh and expects namespaced
results, each with `data`, `updated`, and optional `error` fields. The host
starts one subprocess per request, applies a timeout, and keeps provider
failures isolated from the other readers.

The first example provider reads Frankfurter exchange rates without an API key.
Build it beside the main binary (the default discovery location):

```sh
go build -o bin/gryphdash-provider-currency ./cmd/gryphdash-provider-currency
GRYPHDASH_PROVIDER_DIR="$PWD/bin" ./bin/gryphdash
```

It publishes its EUR/USD, EUR/GBP, GBP/EUR, and EUR/HUF widgets. Override the pairs with
`GRYPHDASH_CURRENCY_PAIRS=USD/EUR,EUR/JPY`, or point tests at a fixture with
`GRYPHDASH_CURRENCY_URL`. Provider executables own their API calls, response
parsing, source names, and credentials; the collector only understands the
generic subprocess protocol.

Codex is also distributed as an external provider executable:

```sh
go build -o bin/gryphdash-provider-codex ./cmd/gryphdash-provider-codex
```

The Windows build helper produces both provider executables alongside the
desktop binary. `GRYPHDASH_CODEX_BIN` is inherited by the Codex provider process.

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
| `GRYPHDASH_DESKTOP_ADDR` | `127.0.0.1:8081` | Desktop HTTP listen address; must remain loopback-only |
| `GRYPHDASH_CODEX_BIN` | `codex` | CLI executable name or full path (not shell arguments) |
| `GRYPHDASH_PROVIDER_DIR` | executable directory or `./bin` | Directory containing `gryphdash-provider-*` external provider executables |
| `GRYPHDASH_PROVIDER_CACHE` | OS cache (`gryphdash/providers`) | Managed provider cache; managed executables take precedence over local providers with the same name |
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

See [LOGO.md](LOGO.md) for the logo's provenance, license, and preparation process.

## Desktop development

The Wails v2 desktop command owns the collector and a loopback-only dashboard
server, then opens that server through a native WebView:

### Prerequisites

Install the Wails v2 CLI at the repository's pinned version:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
```

Linux builds require Go, a C compiler, `pkg-config`, GTK 3 development files,
and WebKitGTK development files. Package names vary by distribution; on
Debian/Ubuntu these are typically provided by packages such as `build-essential`,
`pkg-config`, `libgtk-3-dev`, and `libwebkit2gtk-4.0-dev` (or the WebKitGTK
version supplied by the distribution). Linux tray support also requires a
desktop environment with a compatible system tray implementation.

Windows builds require the Microsoft WebView2 Runtime on the target machine.
Windows 11 normally includes it, but older or managed systems may need it
installed separately. macOS builds require Xcode Command Line Tools. Wails
does not bundle these operating-system WebView dependencies.

```sh
cd cmd/gryphdash-desktop
wails dev
wails build
```

The default `wails build` target is the host platform. Explicit targets can be
selected from the desktop command directory, for example:

```sh
wails build -platform linux/amd64
wails build -platform windows/amd64
wails build -platform darwin/universal
```

Cross-compiling the application is not equivalent to validating a native Wails
build: platform GUI libraries, WebView runtimes, tray behavior, and signing
toolchains still need to be checked on their target operating systems.

The direct Windows Go build must include Wails' `desktop` and `production`
tags, plus the Windows GUI linker flag:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags desktop,production \
  -ldflags "-w -s -H windowsgui" \
  -o bin/gryphdash-desktop-windows-amd64.exe ./cmd/gryphdash-desktop
```

From Linux, the Wails build script packages the Windows desktop executable with
the checked-in Gryph logo and cross-compiles the currency provider alongside it:

```sh
./build-windows.sh
```

The desktop app uses the same Gryph logo for its native Linux and macOS window
icons, and Wails uses `cmd/gryphdash-desktop/build/appicon.png` when packaging
the platform application. Linux application menus may additionally require a
`.desktop` launcher that references the icon.

The desktop server uses `GRYPHDASH_DESKTOP_ADDR`, defaulting to
`127.0.0.1:8081`. The Wails WebView proxies the same dashboard HTTP origin,
including the existing JSON API and browser layout storage.

Desktop data locations use the operating system's user configuration directory:

| Platform | Settings and logs directory |
| --- | --- |
| Linux | `$XDG_CONFIG_HOME/gryphdash` or `~/.config/gryphdash` |
| macOS | `~/Library/Application Support/gryphdash` |
| Windows | `%AppData%\\gryphdash` |

Settings are stored in `settings.json`. Runtime diagnostics are written to
`logs/gryphdash.log` and mirrored to standard error. Logs rotate at 5 MiB and
retain three backups. `GRYPHDASH_LOG_LEVEL` accepts `debug`, `info` (the
default), `warn`, or `error`. Provider responses, credentials, and API keys are
not written to the log.

The desktop application enforces one running instance using the stable
identifier `com.gryphdash.desktop`. Launching GryphDash again shows the existing
window instead of opening a second desktop window.

Closing the desktop window hides it to the system tray while the collector and
dashboard server continue running. Use the tray icon to show the dashboard,
refresh data immediately, or quit GryphDash. The native application menu keeps
Refresh Now available without adding window-level Show or Quit controls. Quit
is the explicit shutdown path and cleanly stops the tray, server, collector,
and provider subprocesses. Native tray support is included in Wails builds
using the `desktop` build tag; ordinary repository tests use a portable stub.

The desktop app stores its settings in `settings.json` under the OS user
configuration directory (for example, `~/.config/gryphdash/settings.json` on
Linux). The dashboard exposes launch-at-login and native notification toggles
only inside Wails; ordinary browser sessions do not request or store these
desktop settings. Launch-at-login uses the platform's native startup mechanism,
and notifications cover provider failures, recovery, and data that remains
stale for several minutes.

The tray provides Show Dashboard, Refresh Now, and Quit. The File menu provides
Refresh Now, Preferences, Open Configuration, and Open Logs. Quit is available
from the tray, while normal window closing hides the window when Close to tray
is enabled.

Installers, code signing, macOS notarization, and automatic updates are
deliberately out of scope for the current development builds. The optional
Windows build helper produces an unsigned binary/package for manual testing;
it is not an installer or update channel.
