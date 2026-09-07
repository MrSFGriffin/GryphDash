# GryphDash

A simple personal dashboard for Codex usage and rate limits, and OpenRouter spend,
budget, and credits. This initial scaffold displays **fictional dummy data only**.
It makes no third-party requests and needs no API tokens.

The server uses only the Go standard library. HTML and CSS are embedded in the
binary: no Node.js, external assets, or runtime template files are required.

## Run

Install Go 1.22 or later, then run from the repository root:

```sh
go run .
```

Open <http://127.0.0.1:8080>. Stop the server with Ctrl+C.

To use another address or port:

```sh
GRYPHDASH_ADDR=127.0.0.1:9090 go run .
```

The default listener is local only. There is no authentication; binding to
`0.0.0.0:8080` makes the dashboard accessible through other network interfaces.

## Build a standalone executable

```sh
go build -o bin/gryphdash .
./bin/gryphdash
```

The binary can run from any directory without the source tree. Rebuild it after
changing the embedded page in `web/dashboard.html`.

## Configuration and future integrations

Currently, `GRYPHDASH_ADDR` is the only environment variable read by the program.
Provider credentials are not implemented yet. A `.env` file is not automatically
loaded; use shell environment variables when configuration is added. `.env` is
gitignored to help keep future local credentials out of version control.

See [PLAN.md](PLAN.md) for integration milestones, including checking which Codex
metrics are available through supported interfaces before selecting credentials.
