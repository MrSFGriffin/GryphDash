package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"gryphdash/internal/app"
	"gryphdash/internal/collector"
	"gryphdash/internal/config"
	"gryphdash/internal/desktop"
	"gryphdash/internal/logging"
	webhandler "gryphdash/internal/web"
	codexprovider "gryphdash/providers/codex"
	openrouterprovider "gryphdash/providers/openrouter"
	webassets "gryphdash/web"
)

const desktopStartupTimeout = 5 * time.Second
const desktopRefreshCompleteEvent = "gryphdash:refresh-complete"
const desktopApplicationID = "com.gryphdash.desktop"

// gryphDashIcon is used by Wails for the native Linux window icon. The same
// source image is also used by the Wails packager for macOS and Windows icons.
//
//go:embed build/appicon.png
var gryphDashIcon []byte

// Windows' tray API requires an ICO file. The PNG remains the source for
// Wails/Linux/macOS, while the multi-size ICO is selected by tray_icon_windows.go.
//
//go:embed build/appicon.ico
var gryphDashIconICO []byte

type wailsWindow struct{}

func (wailsWindow) Show(ctx context.Context) { wailsruntime.Show(ctx) }
func (wailsWindow) Hide(ctx context.Context) { wailsruntime.Hide(ctx) }
func (wailsWindow) Quit(ctx context.Context) { wailsruntime.Quit(ctx) }

func main() {
	closeLogs, logErr := logging.Setup("gryphdash")
	if logErr != nil {
		log.Printf("logging setup failed: %v", logErr)
	} else {
		defer func() { _ = closeLogs() }()
	}
	cfg, err := config.LoadDesktop()
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	collectorInstance := collector.New(collector.Options{
		Providers: []collector.Reader{
			codexprovider.NewAdapter(cfg.CodexExecutable),
			openrouterprovider.NewAdapter(cfg.OpenRouterKey, &http.Client{Timeout: 15 * time.Second}),
		},
	})
	serverRuntime, err := app.NewDesktop(app.Options{
		Collector: collectorInstance,
		Handler:   webhandler.NewHandler(collectorInstance, webassets.FS),
		Address:   cfg.DesktopAddress,
		Interval:  cfg.RefreshInterval,
	})
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	settingsPath, err := desktop.SettingsPath("gryphdash")
	if err != nil {
		log.Printf("desktop settings path: %v", err)
	}
	settings := desktop.DefaultSettings()
	if settingsPath != "" {
		if loaded, loadErr := desktop.LoadSettings(settingsPath); loadErr != nil {
			log.Printf("desktop settings: %v", loadErr)
		} else {
			settings = loaded
		}
	}
	executable, err := os.Executable()
	if err != nil {
		executable = os.Args[0]
	}
	launchAtLogin, err := desktop.NewLaunchAtLogin(desktopApplicationID, executable)
	if err != nil {
		log.Printf("launch at login: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtimeErrors := make(chan error, 1)
	go func() { runtimeErrors <- serverRuntime.Run(ctx) }()
	if err := waitForServer(cfg.DesktopAddress, runtimeErrors); err != nil {
		shutdownDesktopRuntime(serverRuntime)
		log.Print(err)
		os.Exit(1)
	}

	target, err := url.Parse("http://" + cfg.DesktopAddress)
	if err != nil {
		shutdownDesktopRuntime(serverRuntime)
		log.Print(err)
		os.Exit(1)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	var wailsContext context.Context
	wailsContextReady := make(chan struct{})
	contextFn := func() context.Context {
		<-wailsContextReady
		return wailsContext
	}
	refreshNow := func() {
		go func() {
			serverRuntime.Collector().Refresh(context.Background())
			if ctx := contextFn(); ctx != nil {
				wailsruntime.EventsEmit(ctx, desktopRefreshCompleteEvent)
			}
		}()
	}
	windowController := desktop.NewController(wailsWindow{}, settings.CloseToTray, desktop.Actions{
		Refresh: func(context.Context) { refreshNow() },
	})
	desktopBridge := &DesktopBridge{controller: windowController, contextFn: contextFn, settingsPath: settingsPath, settings: settings, launchAtLogin: launchAtLogin}
	notificationMonitor := desktop.NewNotificationMonitor(wailsNotifier{}, 30*time.Minute, 5*time.Minute, desktopBridge.NotificationPreferences, desktopBridge.NotificationScope)
	go runTray(gryphDashIcon, windowController, contextFn, func() {})
	nativeMenu := menu.NewMenu()
	desktopMenu := nativeMenu.AddSubmenu("File")
	desktopMenu.AddText("Refresh Now", nil, func(*menu.CallbackData) {
		windowController.Refresh(contextFn())
	})
	desktopMenu.AddText("Preferences…", nil, func(*menu.CallbackData) {
		wailsruntime.EventsEmit(contextFn(), "gryphdash:open-preferences")
	})
	desktopMenu.AddText("Open Configuration", nil, func(*menu.CallbackData) {
		if err := desktopBridge.OpenConfiguration(); err != nil {
			log.Printf("open configuration: %v", err)
		}
	})
	desktopMenu.AddText("Open Logs", nil, func(*menu.CallbackData) {
		if err := desktopBridge.OpenLogs(); err != nil {
			log.Printf("open logs: %v", err)
		}
	})
	err = wails.Run(&options.App{
		Title:             "GryphDash",
		Width:             1200,
		Height:            800,
		MinWidth:          800,
		MinHeight:         600,
		HideWindowOnClose: false,
		AssetServer:       &assetserver.Options{Handler: proxy},
		Linux:             &linux.Options{Icon: gryphDashIcon},
		Menu:              nativeMenu,
		Bind:              []interface{}{desktopBridge},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: desktopApplicationID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				windowController.Show(contextFn())
			},
		},
		OnBeforeClose: func(ctx context.Context) bool {
			return windowController.BeforeClose(ctx)
		},
		OnStartup: func(ctx context.Context) {
			wailsContext = ctx
			close(wailsContextReady)
			wailsruntime.WindowSetSystemDefaultTheme(ctx)
			if err := wailsruntime.InitializeNotifications(ctx); err != nil {
				log.Printf("desktop notifications: %v", err)
			} else {
				go notificationMonitor.Run(ctx, collectorInstance.Snapshot, 15*time.Second)
			}
		},
		OnShutdown: func(shutdownContext context.Context) {
			wailsruntime.CleanupNotifications(shutdownContext)
			stopTray()
			if err := serverRuntime.Shutdown(shutdownContext); err != nil {
				log.Printf("desktop runtime shutdown: %v", err)
			}
		},
	})
	if err != nil {
		shutdownDesktopRuntime(serverRuntime)
		log.Print(err)
		os.Exit(1)
	}
	if err := <-runtimeErrors; err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func shutdownDesktopRuntime(runtime *app.Runtime) {
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := runtime.Shutdown(shutdownContext); err != nil {
		log.Printf("desktop runtime shutdown: %v", err)
	}
}

func waitForServer(address string, runtimeErrors <-chan error) error {
	deadline := time.Now().Add(desktopStartupTimeout)
	for {
		select {
		case err := <-runtimeErrors:
			if err == nil {
				return fmt.Errorf("desktop HTTP server stopped before the window started")
			}
			return err
		default:
		}
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("desktop HTTP server did not start on %s: %w", address, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
