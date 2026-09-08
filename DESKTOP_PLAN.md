# Wails desktop application

## Summary

Add a separate Wails v2 desktop application targeting Linux, Windows, and
macOS. It will start and own the existing collectors, display the existing web
dashboard, and provide native desktop integration.

Wails v2 is the selected framework because it is Go-based, uses the platform
WebView, and is currently stable. Wails v3 remains beta.

The first release will provide development builds only. Installers, signing,
notarization, and auto-update are explicitly out of scope.

## Application architecture

Refactor the current root application into reusable packages so the existing
web server, TUI, and desktop app share:

- Collector construction and lifecycle.
- Provider adapters.
- Dashboard snapshot generation.
- Widget catalog loading and validation.
- Configuration and logging.
- HTTP handlers.

Add a separate desktop command, such as:

```text
cmd/gryphdash-desktop/
```

The desktop binary will:

1. Create the collector.
2. Start the collector refresh loop.
3. Start the existing dashboard HTTP handler on loopback.
4. Open the dashboard in a Wails WebView.
5. Keep the collector and HTTP server alive while the window is hidden to the tray.
6. Shut both down only when the user selects Quit from the tray or menu.

Use a stable, configurable loopback address for the desktop server so browser
`localStorage` layouts persist across launches. Default to a dedicated desktop
port, with a clear startup error if it is already occupied. Bind only to
`127.0.0.1`.

The WebView must load the same dashboard origin on every launch. Existing
GridStack layouts, named layouts, widget picker behavior, and dashboard JSON
remain unchanged.

## Native desktop behavior

Implement a platform-neutral desktop controller with OS-specific adapters for:

- Main window show, hide, focus, and close handling.
- Single-instance enforcement.
- System tray icon and menu.
- Tray actions: Show dashboard, Refresh now, Open logs/configuration, and Quit.
- Native application menu.
- Launch-at-login enable/disable setting.
- Native notifications for collector failures, recovery, and prolonged stale data.
- Theme-aware window behavior where supported.
- Opening the local dashboard or diagnostics directory in the system file/browser application.

Closing the window minimizes to the tray by default. The application continues
collecting while hidden. The tray Quit action performs orderly context
cancellation, server shutdown, collector shutdown, and process exit.

Persist desktop-specific settings in the OS-appropriate user configuration
directory. Keep credentials in the environment/configuration handling already
used by the collector; never expose them to JavaScript or store them in browser
storage.

## Frontend and backend boundary

Keep the existing HTTP/JSON dashboard API as the shared UI boundary.
Desktop-specific features should be exposed through a small Wails bridge only
where native behavior is required, such as:

- Refresh now.
- Show/hide/quit window.
- Launch-at-login setting.
- Notification permission/status.
- Open logs or configuration.

The dashboard renderer must continue to work as a normal browser application
without Wails APIs. Feature detection should leave those controls hidden or use
browser-safe behavior when running in regular web mode.

## Build and packaging

Add Wails v2 project metadata and development commands without adding installer
generation yet.

Document:

- Wails prerequisites for Linux, Windows, and macOS.
- Platform-specific WebView requirements.
- Local development command.
- Desktop build commands for each target.
- Configuration and data-directory locations.
- Tray and close-to-tray behavior.
- The absence of signing, notarization, installers, and auto-update in this phase.

Release automation may produce unsigned platform binaries for manual testing,
but no installer or update channel should be designed until the desktop behavior
is stable.

## Test plan

Add unit and integration coverage for:

- Shared collector startup and shutdown.
- Desktop configuration and stable loopback address selection.
- Port-in-use and startup failure reporting.
- Single-instance behavior through a testable abstraction.
- Close-to-tray versus explicit Quit behavior.
- Tray action dispatch.
- Notification transition and rate-limiting logic using mocks.
- Launch-at-login adapter behavior using mocks.
- Dashboard API serving from the desktop-owned collector.
- Preservation of browser layout storage across desktop restarts at the stable origin.
- Clean shutdown when the application receives an OS termination signal.
- Provider failures remaining isolated and visible as stale/unavailable states.

Run the normal Go formatting, test, vet, build, JavaScript syntax, and diff
checks. Browser automation remains optional manual QA and is not part of the
implementation workflow.

## Assumptions

- Wails v2 is used for the first desktop implementation.
- Linux, Windows, and macOS are all supported targets.
- The desktop app owns the collector and does not require a separately running
  web server.
- Closing the window minimizes to the tray by default.
- The tray provides the explicit Quit path.
- The desktop HTTP server is loopback-only with a stable configurable port.
- Existing web and TUI modes remain available.
- Installers, code signing, notarization, and auto-update are deferred.
