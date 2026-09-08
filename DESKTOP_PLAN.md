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

## Implementation steps

### Phase 1: Define desktop boundaries

1. Choose the Wails v2 version and application identifier.
2. Define the desktop configuration model:
   - Loopback address and dedicated default port.
   - Configuration and data directories.
   - Launch-at-login setting.
   - Close-to-tray preference.
3. Define interfaces for platform-dependent behavior:
   - Window control.
   - Tray and menu.
   - Single-instance locking.
   - Notifications.
   - Launch-at-login.
   - Opening files and directories.
4. Decide which settings are environment variables and which are persisted desktop settings.

Deliverable: a desktop architecture decision document and interface definitions.

### Phase 2: Extract reusable application packages

5. Extract configuration parsing and defaults.
6. Extract collector construction and lifecycle into a reusable package.
7. Extract dashboard snapshot generation and widget catalog validation.
8. Extract embedded dashboard assets and HTTP handler creation.
9. Extract a shared application runtime that owns the collector, refresh loop,
   HTTP server, context cancellation, and graceful shutdown.
10. Update the existing web and TUI commands to use the shared packages.

Deliverable: existing `web` and `tui` modes continue to behave as before.

### Phase 3: Add the desktop-owned HTTP server

11. Add desktop-specific server configuration.
12. Bind only to `127.0.0.1`.
13. Use a stable, configurable desktop port.
14. Return a clear startup error when the port is occupied.
15. Ensure the same origin is used on every launch so browser `localStorage`
    layouts survive restarts.
16. Add tests for address selection, port conflicts, and shutdown.

Deliverable: the dashboard runs under a desktop-owned collector and server.

### Phase 4: Create the Wails desktop shell

17. Add `cmd/gryphdash-desktop/`.
18. Add Wails project metadata and development commands.
19. Start the shared runtime from the desktop command.
20. Load the dashboard URL in the Wails WebView.
21. Keep the application alive while the window is hidden.
22. Implement orderly shutdown of the Wails window, HTTP server, collector
    refresh loop, and provider subprocesses.

Deliverable: a basic desktop window displays the existing dashboard.

### Phase 5: Implement native desktop behavior

23. Implement the platform-neutral desktop controller.
24. Add window actions for showing, hiding, focusing, and intercepting close.
25. Make normal window closing hide the application to the tray.
26. Add explicit Quit behavior that performs full shutdown.
27. Add single-instance enforcement through a testable abstraction.
28. Add the tray icon and menu with actions for showing the dashboard,
    refreshing now, opening logs or configuration, and quitting.
29. Add the native application menu.
30. Add OS-specific adapters for Linux, Windows, and macOS.

Deliverable: the desktop app behaves consistently across supported platforms.

### Phase 6: Add desktop settings and notifications

31. Persist settings in the platform-appropriate user configuration directory.
32. Implement launch-at-login enable/disable through an adapter.
33. Implement opening logs and configuration directories.
34. Add native notifications for collector failure, recovery, and prolonged
    stale data.
35. Add notification transition and rate-limiting logic.
36. Add theme-aware behavior where the platform supports it.

Deliverable: native integration works without exposing credentials to the WebView.

### Phase 7: Add the small Wails bridge

37. Expose only native-required actions:
   - Refresh now.
   - Show, hide, and quit the window.
   - Read and change launch-at-login status.
   - Read notification status.
   - Open logs and configuration.
38. Add feature detection in `dashboard.js`.
39. Keep native controls hidden or browser-safe when running outside Wails.
40. Preserve the existing dashboard API, widget IDs, layouts, and browser
    storage keys.

Deliverable: the same frontend works in both a normal browser and the desktop app.

### Phase 8: Testing

41. Add shared runtime startup and shutdown tests.
42. Add desktop configuration and stable-address tests.
43. Add port-in-use tests.
44. Add single-instance tests using a fake adapter.
45. Add close-to-tray and explicit Quit tests.
46. Add tray action dispatch tests.
47. Add notification transition and rate-limiting tests.
48. Add launch-at-login adapter tests.
49. Add dashboard API integration tests using a desktop-owned collector.
50. Test layout persistence across desktop restarts at the same origin.
51. Test OS termination signal handling.
52. Confirm provider failures remain isolated and appear as stale or unavailable data.

### Phase 9: Documentation and builds

53. Document Wails prerequisites for Linux, Windows, and macOS.
54. Document WebView requirements.
55. Document development and platform build commands.
56. Document configuration and data-directory locations.
57. Document tray and close-to-tray behavior.
58. Explicitly document that installers, signing, notarization, and auto-update
    are deferred.
59. Add optional unsigned binary build automation only after local behavior is stable.

### Recommended milestone sequence

1. Shared runtime extraction.
2. Desktop-owned HTTP server.
3. Basic Wails window.
4. Graceful shutdown.
5. Tray and close-to-tray behavior.
6. Single-instance enforcement.
7. Native bridge and frontend feature detection.
8. Settings, notifications, and launch-at-login.
9. Platform adapters.
10. Documentation, cross-platform builds, and final validation.
