# System tray becomes unresponsive: investigation notes

Investigated 2026-09-11 against commit `619c51f`. The Windows thread-affinity finding below was addressed after this investigation by adding `runtime.LockOSThread` around the lifetime of the Windows tray goroutine (`cmd/gryphdash-desktop/tray_windows.go`). It still requires Windows runtime QA to confirm it resolves the reported incident.

## Assessment

The strongest lead was incorrect native event-loop/thread ownership in the Windows tray integration. `main.go:170` starts `go runTray(...)`, while Wails runs on the main goroutine. Before the follow-up fix, the tray goroutine was not pinned to an OS thread. Windows requires the tray's native window and its message pump to stay on the same thread. Linux has a separate concern: both libraries initialize GTK and run its main loop from different goroutines.

There is also a startup-order problem that can prevent launching a second copy from showing the existing window. This could turn a tray failure into the reported inability to recover the app.

These are source-backed findings, not a reproduced diagnosis of the reported incident. The affected OS, executable version/build tags, trigger, and process state during a freeze were not supplied. No native app was launched or attached to during this investigation.

## 1. Windows: tray message pump could lose its window's thread

Relevant repository code:

- `cmd/gryphdash-desktop/main.go:170`: starts the tray in a new goroutine.
- Before the follow-up fix, `cmd/gryphdash-desktop/tray.go`, `runTray` called `systray.Run` without `runtime.LockOSThread`. The Windows implementation now lives in `tray_windows.go` and locks its OS thread around `systray.Run`.
- `go.mod`: pins `github.com/getlantern/systray v1.2.2` and Wails `v2.15.0`.

The locally cached systray v1.2.2 source shows:

- `systray.go`, `init`: calls `runtime.LockOSThread`, but this pins the startup/main goroutine, **not** a subsequently spawned tray goroutine.
- `systray.go`, `Run`: calls `Register` and then `nativeLoop`.
- `systray_windows.go`, `registerSystray` / `initInstance`: creates the hidden native window using `CreateWindowExW`.
- `systray_windows.go:781`, `nativeLoop`: repeatedly calls `GetMessageW` with a null window handle, followed by message translation/dispatch. Neither initialization nor this loop pins the goroutine.
- `systray_windows.go:248`, `wndProc`: left/right button-up notifications call `showMenu`. Menu opening therefore depends on native message dispatch, before the application's `ClickedCh` handler is involved.

Go permits the unpinned goroutine to resume on another OS thread between native calls. The tray window stays owned by its original thread. Microsoft's `GetMessageW` documentation says that a null window argument retrieves messages for the calling thread's windows/queue. A pump running on a different thread can therefore stop receiving tray clicks while the icon remains visible. Migration after initially successful clicks is a plausible explanation for intermittent failure after startup; migration during initialization could produce an immediately inert tray.

Confidence: high that the required thread guarantee was missing; the specific migration/freeze still needs runtime confirmation. A separate Windows tray thread is not inherently wrong—the missing lifetime thread affinity was the concern.

Sources: [Go runtime.LockOSThread](https://pkg.go.dev/runtime#LockOSThread), [Microsoft GetMessageW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getmessage).

## 2. Linux: two GTK owners/main loops

In systray v1.2.2, `systray_linux.c`, `registerSystray` calls `gtk_init`, constructs the indicator/menu, and `nativeLoop` calls `gtk_main`. These run through the background tray goroutine. Wails v2.15.0, `internal/frontend/desktop/linux/frontend.go`, independently initializes GTK in `NewFrontend` and calls `gtk_main` in `RunMainLoop`.

This creates unsynchronized GTK initialization and competing main-loop ownership in one process. Wails also uses `g_idle_add` to dispatch GUI work and records the first dispatch thread as `mainTid` (`linux/invoke.go`); the tray uses the default context for idle callbacks too. Those paths deserve inspection together if the affected build is Linux. GTK documents that GTK/GDK calls belong on the main thread. Do not assume that adding a thread lock to the tray goroutine alone resolves the Linux integration.

Confidence: high that the integration violates the expected single GUI-thread model; no specific GTK deadlock or stack was observed. A panel/AppIndicator/session problem remains another possibility if failures correlate with shell restart, suspend/resume, or a particular desktop environment.

Source: [GTK 3 threading guidance](https://docs.gtk.org/gdk3/func.threads_init.html).

The macOS backend also warrants a platform-specific review: systray's `systray_darwin.m` installs an `NSApplication` delegate and runs `NSApp`, while this application starts it alongside Wails. This was not pursued as an incident diagnosis without knowing the affected platform.

## 3. Relaunch can fail before single-instance activation

In `cmd/gryphdash-desktop/main.go`, startup runs `serverRuntime.Run` and `waitForServer` **before** reaching `wails.Run` and `SingleInstanceLock`. The HTTP runtime attempts to bind the configured address (`internal/app/runtime.go`, `Run`), normally `127.0.0.1:8081`.

When the first instance still owns that port, the second instance's server reports a listen failure. `waitForServer` races between two observations:

1. If it sees `runtimeErrors` first, startup exits before Wails can invoke `OnSecondInstanceLaunch` in the existing instance.
2. If its TCP probe connects to the existing server first, it accepts that as readiness and proceeds to Wails. The probe does not verify that the new process owns the listener.

Thus relaunch is timing-dependent even if the first instance's Wails UI is healthy. This is independent of why the tray stopped responding, but directly relevant to the lack of a recovery path. Look for `HTTP server cannot listen on ...` in the second launch's logs. Provider discovery also happens before single-instance handling, so relaunch can be delayed there before reaching either outcome.

Confidence: source-confirmed control-flow race; not reproduced by launching another desktop process here.

## Other paths checked

- Refresh work is launched in another goroutine (`main.go`, `refreshNow`). A slow provider does not directly occupy the tray's menu-opening path.
- The tray's single action consumer could stop handling subsequent selections if `Show`/`Quit` or `contextFn` blocks. However, systray sends menu-selection notifications nonblockingly and drops them when no receiver is ready (`systrayMenuItemSelected`). That alone does not explain a native menu that never opens.
- `contextFn` waits indefinitely for `OnStartup`, but the readiness channel stays closed once startup has completed. It is not a strong explanation for an app that worked normally before freezing.
- `Controller.BeforeClose` holds a mutex across `window.Hide`; this deserves consideration if a stack shows a blocked hide, but `Controller.Show` does not acquire that mutex. No controller deadlock was established.
- The tray has no application-level readiness/health reporting. Its exit callback is empty, and return from `runTray` is not monitored. Close-to-tray continues hiding the window regardless of tray health. Some native failures are logged by systray, but their capture in the application's log file was not verified.
- Windows systray already handles `TaskbarCreated` by re-adding the icon. An Explorer restart should not be blamed on a completely absent handler; the handler itself still needs a functioning message pump.

## Evidence to collect on the next occurrence

1. Record OS/version, desktop environment if applicable, executable version, and whether the tray ever worked in that process. Record whether failure followed hiding, idle time, refresh, sleep/wake, or a shell restart. Distinguish “menu never opens” from “menu opens but Show Dashboard does nothing.”
2. Check whether the original process is alive. Query the configured loopback server with a short timeout, for example `curl --max-time 3 http://127.0.0.1:8081/api/widgets`. A responding server with an inert tray points toward the native UI path; a dead process could leave a stale shell icon. A failed HTTP request alone does not prove a tray-thread problem.
3. Preserve `logs/gryphdash.log` and rotated files under the OS user configuration directory's `gryphdash` folder, plus any available stderr/native diagnostics. Note timestamps of clicks and relaunch attempts. Current code does not log thread IDs or each tray click, so an empty log does not rule out this hypothesis.
4. For Windows, capture native thread stacks and compare the tray HWND's owner thread (`GetWindowThreadProcessId`) with the thread pumping `GetMessageW`. For Linux, inspect GTK/main-context stacks and which threads initialized and execute GUI work. A native dump/debugger is needed to confirm the leading explanation; ordinary Go tests cannot establish native thread ownership.
5. Separately exercise repeated second-instance launches while the first process is healthy and hidden, recording port-bind errors and whether its window appears. This separates recovery failure from tray failure.

Future fix investigation should start with platform-appropriate event-loop ownership, then move single-instance activation ahead of exclusive server startup. systray exposes `Register` for integration with another GUI loop, but its platform implementations differ; replacing `Run` blindly is not a verified cross-platform fix. No such changes were made.

## Validation and limits

`go test ./internal/desktop ./internal/app` passed (cached results). The initial sandboxed Go launcher failed due to snap capability restrictions; focused tests succeeded outside that sandbox. Controller tests use fake windows, and ordinary desktop command builds select `tray_stub.go` without the `desktop` tag, so these checks do not cover native tray behavior. No browser tooling, live provider accounts, GUI reproduction, or implementation changes were used. Full code-change checks were not run because this change only adds investigation notes.
