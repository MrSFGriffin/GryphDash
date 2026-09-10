# Repository-backed provider distribution

## Summary

Add support for discovering and installing providers from separate HTTPS-hosted provider repositories.

Repositories are explicitly trusted by the user. There will be no publisher-signature or central-approval system in v1.

## Provider repository format

- Define a versioned HTTPS repository index containing:
  - Repository metadata.
  - Provider ID, name, description, version, and protocol version.
  - Widget catalog metadata.
  - Platform-specific executable URLs and SHA-256 checksums.
- Support release binaries for Linux amd64, Windows amd64, macOS amd64, and macOS arm64.
- Treat repository URLs as index endpoints rather than requiring Git or a hosting-service API.
- Require HTTPS for indexes and binary downloads.
- Use SHA-256 to detect corruption or mismatched downloads, while clearly documenting that it does not establish publisher authenticity.

## Configuration and lifecycle

- Add a built-in default core-provider repository URL.
- Store additional repository URLs in the existing OS configuration directory.
- Add repository management and provider Install, Update, and Remove actions to Web, Desktop, and TUI.
- Adding a repository shows a warning that it can supply arbitrary native executables.
- Repository configuration alone only fetches metadata; binaries download only after explicit installation.
- Updates are manual and explicit.
- Cache providers by repository, provider, version, and platform.
- Download to a temporary file, verify SHA-256, set executable permissions, and atomically install only after verification succeeds.
- Preserve already-installed providers when a repository becomes unavailable.

## Discovery and compatibility

- Extend provider discovery to include the managed provider cache.
- Use only provider executables discovered from the managed cache or an explicitly configured external provider directory.
- Prefer an installed managed provider over an explicitly configured external local provider with the same provider name.
- Use an external local provider when no managed provider for that name is installed.
- Reject duplicate provider names from competing configured repositories unless they represent the same installed source/version.
- Continue rejecting duplicate widget IDs and invalid catalogs.
- Keep the existing subprocess protocol, timeout, stderr capture, environment handling, and process cleanup.

## Separate core-provider repository

Create a separate repository for the core provider implementations and release artifacts, initially containing the existing Codex, Currency, and OpenRouter providers.

For local development, place that repository at `~/src/GryphDash-Providers`.

Update the build scripts and documentation to require the external provider repository for local provider builds.

## Testing and acceptance criteria

- Manifest parsing and schema validation tests.
- HTTPS enforcement and platform-selection tests.
- SHA-256 success, mismatch, truncated-download, and failed-download tests using local HTTP fixtures.
- Atomic-install and cache-recovery tests.
- Repository configuration persistence tests.
- Tests proving metadata discovery does not download or execute binaries.
- Tests for explicit install, update, removal, and repository outage.
- Tests for provider-name and widget-ID conflicts.
- Existing subprocess fixture tests remain passing.
- Run Go formatting, tests, vet, build, race tests, JavaScript checks, shell syntax checks, and `git diff --check`.

## Assumptions

- Repository trust is opt-in and intentionally permissive.
- No signatures, signing keys, publisher verification, or central registry are required in v1.
- SHA-256 is an integrity check only.
- Installation is always explicit.
- All three interfaces support repository management and installation.
- Provider implementations and releases live in the separate provider repository.
