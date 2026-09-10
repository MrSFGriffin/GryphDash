# Repository-backed provider distribution

## Summary

Add support for discovering and installing providers from separate HTTPS-hosted provider repositories while keeping the current in-repo providers working as a fallback.

Repositories are explicitly trusted by the user. There will be no publisher-signature or central-approval system in v1.

## Provider repository format

- Define a versioned HTTPS manifest format containing:
  - Repository metadata.
  - Provider ID, name, description, version, and protocol version.
  - Widget catalog metadata.
  - Platform-specific executable URLs and SHA-256 checksums.
- Support release binaries for Linux amd64, Windows amd64, macOS amd64, and macOS arm64.
- Treat repository URLs as manifest endpoints rather than requiring Git or a hosting-service API.
- Require HTTPS for manifests and binary downloads.
- Use SHA-256 to detect corruption or mismatched downloads, while clearly documenting that it does not establish publisher authenticity.

## Configuration and lifecycle

- Add a built-in default core-provider repository URL.
- Store additional repository URLs and enabled/disabled state in the existing OS configuration directory.
- Add repository management and provider Install, Update, and Remove actions to Web, Desktop, and TUI.
- Adding a repository shows a warning that it can supply arbitrary native executables.
- Repository configuration alone only fetches metadata; binaries download only after explicit installation.
- Updates are manual and explicit.
- Cache providers by repository, provider, version, and platform.
- Download to a temporary file, verify SHA-256, set executable permissions, and atomically install only after verification succeeds.
- Preserve already-installed providers when a repository becomes unavailable.

## Discovery and compatibility

- Extend provider discovery to include the managed provider cache.
- Keep the existing in-repository provider executables and source/build workflow functional as fallback.
- Prefer an installed managed provider over an in-repo provider with the same provider name.
- Use the in-repo provider when no managed provider for that name is installed.
- Reject duplicate provider names from competing configured repositories unless they represent the same installed source/version.
- Continue rejecting duplicate widget IDs and invalid catalogs.
- Keep the existing subprocess protocol, timeout, stderr capture, environment handling, and process cleanup.

## Separate core-provider repository

Create a separate repository for the core provider implementations and release artifacts, initially containing the existing Codex, Currency, and OpenRouter providers.

For local development, place that repository at `~/src/GryphDash-Providers`.

The dashboard repository will continue to contain the current providers during this transition, so local development and existing builds remain usable if the external core repository is unavailable.

Update the build scripts and documentation to explain both paths:

- local in-repo providers for development/fallback;
- repository-installed providers for normal distribution and additional third-party providers.

## Testing and acceptance criteria

- Manifest parsing and schema validation tests.
- HTTPS enforcement and platform-selection tests.
- SHA-256 success, mismatch, truncated-download, and failed-download tests using local HTTP fixtures.
- Atomic-install and cache-recovery tests.
- Repository configuration persistence tests.
- Tests proving metadata discovery does not download or execute binaries.
- Tests for explicit install, update, removal, repository outage, and local-provider fallback.
- Tests for provider-name and widget-ID conflicts.
- Existing subprocess and provider fixture tests remain passing.
- Run Go formatting, tests, vet, build, race tests, JavaScript checks, shell syntax checks, and `git diff --check`.

## Assumptions

- Repository trust is opt-in and intentionally permissive.
- No signatures, signing keys, publisher verification, or central registry are required in v1.
- SHA-256 is an integrity check only.
- Installation is always explicit.
- All three interfaces support repository management and installation.
- Existing in-repo providers remain available throughout this implementation.
