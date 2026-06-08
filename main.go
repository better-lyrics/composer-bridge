package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/boidushya/composer-bridge/internal/activity"
	"github.com/boidushya/composer-bridge/internal/app"
	"github.com/boidushya/composer-bridge/internal/config"
	"github.com/boidushya/composer-bridge/internal/events"
	"github.com/boidushya/composer-bridge/internal/library"
	"github.com/boidushya/composer-bridge/internal/server"
	"github.com/boidushya/composer-bridge/internal/updater"
	"github.com/boidushya/composer-bridge/internal/ytdlp"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

const Version = "0.1.0"

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

	handlers := &server.Handlers{
		Library:     lib,
		Activity:    act,
		YtdlpPath:   ytdlpPath,
		ThumbDir:    filepath.Join(dataDir, "thumbs"),
		Bridge:      Version,
		AudioFormat: cfg.AudioFormat,
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
	err = wails.Run(&options.App{
		Title:            "Composer Bridge",
		Width:            1024,
		Height:           700,
		MinWidth:         800,
		MinHeight:        540,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 0x28, G: 0x29, B: 0x2c, A: 255},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
		},
		OnStartup: func(ctx context.Context) {
			a.Startup(ctx)
			handlers.EmitterCtx = ctx
		},
		OnShutdown: a.Shutdown,
		Bind:       []any{a},
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
