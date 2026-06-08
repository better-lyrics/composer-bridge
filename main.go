package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/activity"
	"github.com/better-lyrics/composer-bridge/internal/app"
	"github.com/better-lyrics/composer-bridge/internal/autostart"
	"github.com/better-lyrics/composer-bridge/internal/config"
	"github.com/better-lyrics/composer-bridge/internal/events"
	"github.com/better-lyrics/composer-bridge/internal/library"
	"github.com/better-lyrics/composer-bridge/internal/server"
	"github.com/better-lyrics/composer-bridge/internal/updater"
	"github.com/better-lyrics/composer-bridge/internal/ytdlp"
	"github.com/better-lyrics/composer-bridge/tray"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

const Version = "1.0.6"

func main() {
	dataDir := resolveDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fatal("data dir: %v", err)
	}

	cfgPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Warn("config load fell back to defaults", "err", err)
	}

	logPath := filepath.Join(dataDir, "bridge.log")
	if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		defer logFile.Close()
		slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)})))
	}

	ytdlpPath, err := ytdlp.Ensure(dataDir)
	if err != nil {
		fatal("ensure yt-dlp: %v", err)
	}

	lib, err := library.Open(filepath.Join(dataDir, "library.db"))
	if err != nil {
		fatal("open library: %v", err)
	}
	defer lib.Close()

	act, err := activity.Open(filepath.Join(dataDir, "activity.db"))
	if err != nil {
		fatal("open activity: %v", err)
	}
	defer act.Close()

	port, err := server.SelectPort(cfg.ListenPort, dataDir)
	if err != nil {
		fatal("port: %v", err)
	}

	// Cache the yt-dlp version once at startup instead of execing the binary
	// on every /health and every Settings poll. The initial probe runs in a
	// goroutine so a slow / hanging exec doesn't block the HTTP server from
	// binding. RefreshDaily updates the cache on every successful upgrade.
	var ytdlpVersionCache atomic.Pointer[string]
	unknown := "unknown"
	ytdlpVersionCache.Store(&unknown)
	refreshYtdlpVersion := func() {
		v := ytdlp.Version(ytdlpPath)
		ytdlpVersionCache.Store(&v)
	}
	go refreshYtdlpVersion()
	getYtdlpVersion := func() string { return *ytdlpVersionCache.Load() }

	handlers := &server.Handlers{
		Library:      lib,
		Activity:     act,
		YtdlpPath:    ytdlpPath,
		YtdlpVersion: getYtdlpVersion,
		ThumbDir:     filepath.Join(dataDir, "thumbs"),
		Bridge:       Version,
		AudioFormat:  cfg.AudioFormat,
		Emitter: events.EmitterFunc(func(ctx context.Context, name string, args ...any) {
			if ctx == nil {
				return
			}
			wailsRuntime.EventsEmit(ctx, name, args...)
		}),
	}
	httpSrv := &http.Server{
		Handler:           server.WithCORS(handlers.Router(), cfg.AllowedOrigins),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		slog.Info("bridge listening", "url", fmt.Sprintf("http://localhost:%d", port.Port))
		_ = httpSrv.Serve(port.Listener)
	}()

	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	go ytdlp.RefreshDaily(bgCtx, dataDir)
	go updater.PollDaily(bgCtx, updater.DefaultManifestURL, Version, func(info updater.UpdateInfo) {
		slog.Info("bridge update available", "version", info.Latest, "current", info.Current)
	})

	a := app.New(lib, act, cfg, cfgPath, dataDir, ytdlpPath, Version)
	a.SetYtdlpVersionFn(getYtdlpVersion)

	if exec, err := os.Executable(); err == nil {
		if err := autostart.Refresh(exec); err != nil {
			slog.Warn("autostart refresh failed", "err", err)
		}
	}

	trayCtrl := tray.New()
	trayCtrl.Register()

	err = wails.Run(&options.App{
		Title:             "Composer Bridge",
		Width:             1024,
		Height:            700,
		MinWidth:          800,
		MinHeight:         540,
		AssetServer:       &assetserver.Options{Assets: assets},
		BackgroundColour:  &options.RGBA{R: 0x28, G: 0x29, B: 0x2c, A: 255},
		// StartHidden suppresses Wails's default makeKeyAndOrderFront so the
		// brief ~50ms during which Wails's own AppDelegate forces the policy
		// to Regular doesn't produce a Dock-icon flash. OnStartup then calls
		// DockShow + WindowShow once the activation policy is settled.
		StartHidden: true,
		// HideWindowOnClose is false so the X button routes through
		// OnBeforeClose instead of being intercepted by Wails's [NSApp hide:]
		// shortcut, which doesn't drop the Dock icon and bypasses our hook.
		HideWindowOnClose: false,
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "dev.boidu.composer-bridge.single-instance",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				if active := app.Active(); active != nil && active.Ctx() != nil {
					tray.DockShow()
					wailsRuntime.WindowShow(active.Ctx())
				}
			},
		},
		OnStartup: func(ctx context.Context) {
			a.Startup(ctx)
			handlers.EmitterCtx = ctx
			trayCtrl.BindContext(ctx)
			trayCtrl.OnQuit(a.MarkQuitting)
			trayCtrl.Start()
		},
		OnBeforeClose: a.OnBeforeClose,
		OnShutdown: func(ctx context.Context) {
			trayCtrl.Stop()
			a.Shutdown(ctx)
		},
		Bind: []any{a},
	})
	if err != nil {
		fatal("wails: %v", err)
	}
}

func resolveDataDir() string {
	if env := os.Getenv("COMPOSER_BRIDGE_DATA_DIR"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".composer-bridge")
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func parseLogLevel(name string) slog.Level {
	switch name {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
