# Provider repositories

GryphDash provider repositories are HTTPS-hosted manifest endpoints. A
repository can advertise one provider release at a time, including its widget
catalog and platform-specific artifacts.

## Manifest format

The current schema is version 1:

```json
{
  "version": 1,
  "repository": {
    "id": "core",
    "name": "GryphDash Core Providers",
    "description": "First-party providers"
  },
  "provider": {
    "id": "currency",
    "name": "Currency",
    "description": "Reference exchange rates",
    "version": "1.0.0",
    "protocolVersion": 1,
    "widgets": { "widgets": [] },
    "artifacts": {
      "linux-amd64": {
        "url": "https://downloads.example/currency-linux-amd64",
        "sha256": "64-hexadecimal-character-sha256"
      },
      "windows-amd64": {
        "url": "https://downloads.example/currency-windows-amd64.exe",
        "sha256": "64-hexadecimal-character-sha256"
      }
    }
  }
}
```

`version`, repository and provider identifiers, provider metadata,
`protocolVersion`, widget definitions, and artifact checksums are required.
Artifact URLs must be HTTPS and must be supplied for the target platform.
Supported platforms are `linux-amd64`, `windows-amd64`, `darwin-amd64`, and
`darwin-arm64`. Widget IDs must be unique within a provider and across the
combined GryphDash catalog.

Metadata discovery fetches only the manifest. It never downloads or executes an
artifact. Installation downloads the selected artifact, verifies SHA-256,
sets executable permissions, and atomically places it in the managed cache.

## Trust model and setup

Adding a repository is an explicit trust decision: a verified artifact checksum
detects corruption but does not identify its publisher. Repositories can supply
arbitrary native executables, so add only HTTPS endpoints you trust. GryphDash
stores repository settings in the OS configuration directory and includes a
built-in core repository by default.

For local development, clone the core provider repository beside GryphDash:

```sh
git clone https://github.com/MrSFGriffin/GryphDash-Providers ~/src/GryphDash-Providers
GRYPHDASH_PROVIDERS_DIR="$HOME/src/GryphDash-Providers" ./run-web.sh
```

The helper scripts automatically use that sibling checkout when present. If it
is unavailable, they build the identical providers retained in GryphDash as an
offline development fallback. The provider repository's `build-release.sh`
cross-compiles the Codex, Currency, and OpenRouter executables and writes
`SHA256SUMS` for publishing.

## Lifecycle

Repository configuration and provider installation are separate operations:

1. Add an HTTPS manifest endpoint. GryphDash fetches metadata only.
2. Review the repository/provider metadata and trust warning.
3. Install a provider explicitly. Its state becomes `installing`, then
   `installed` after checksum verification.
4. When newer metadata is available, the provider state becomes `update`.
   Updates are explicit and preserve the previous install until the new one is
   verified.
5. Remove a provider explicitly when it is no longer needed. Removing a
   repository configuration does not delete already-installed binaries.

The Web API exposes repository management at `/api/provider-repositories` and
provider lifecycle actions/status at `/api/provider-status`. The desktop bridge
and TUI expose the same operations. Unavailable repositories are shown as
`unavailable`; known metadata retained through an outage is shown as `stale`.

Managed providers take precedence over local executables with the same provider
ID. If no managed copy is installed, GryphDash falls back to executables beside
the application, in `./bin`, or in `GRYPHDASH_PROVIDER_DIR`.
