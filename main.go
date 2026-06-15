package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/activity"
	"github.com/better-lyrics/composer-bridge/internal/app"
	"github.com/better-lyrics/composer-bridge/internal/autostart"
	"github.com/better-lyrics/composer-bridge/internal/bridge"
	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
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

// Version is intentionally a var (not a const) so the release workflow's
// `-ldflags "-X main.Version=$VERSION"` injection actually takes effect at
// link time. Constants are inlined by the compiler and cannot be overridden,
// which is why this stayed in sync with tags only through manual edits.
var Version = "1.4.9"

func main() {
	// SingleInstanceLock handshake: when ApplyAndRelaunch spawns us with the
	// RelaunchUpdatedFlag, the parent's flock file descriptor may still be open
	// for a few microseconds after its os.Exit. Sleep before wails.Run so the
	// kernel has time to release the lock; otherwise our boot trips
	// OnSecondInstanceLaunch (focus a window that is mid-shutdown) and we
	// silently exit. No OSS project pairs minio/selfupdate with Wails'
	// SingleInstanceLock so this 500ms is uncharted but empirically generous.
	if slices.Contains(os.Args[1:], updater.RelaunchUpdatedFlag) {
		time.Sleep(500 * time.Millisecond)
	}

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

	installGoroutineDumpSignal(dataDir)

	if err := bootstrapYtdlp(context.Background(), dataDir, cfg.YtdlpChannel, cfg.YtdlpBinaryPath); err != nil {
		fatal("ensure yt-dlp: %v", err)
	}
	bootstrapDeno(dataDir)

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

	// Cache the yt-dlp version once at startup instead of execing the binary
	// on every /health and every Settings poll. The initial probe runs in a
	// goroutine so a slow / hanging exec doesn't block the HTTP server from
	// binding. The closure takes the path as an argument so daily/once/force
	// upgrades can refresh against the binary they just installed rather than
	// the boot-time path, which would be stale if the user had a binary-path
	// override set at boot.
	var ytdlpVersionCache atomic.Pointer[string]
	unknown := "unknown"
	ytdlpVersionCache.Store(&unknown)
	refreshYtdlpVersion := func(path string) {
		v := ytdlp.Version(path)
		ytdlpVersionCache.Store(&v)
	}
	getYtdlpVersion := func() string { return *ytdlpVersionCache.Load() }

	holder := bridgestate.NewHolder()

	a := app.New(lib, act, cfg, cfgPath, dataDir, Version)
	// Probe the boot-time path off-thread so a slow exec doesn't block the
	// HTTP listener. Resolves through the App so a mid-session override flip
	// is honored on subsequent refreshes.
	go refreshYtdlpVersion(a.GetYtdlpPath())

	handlers := &server.Handlers{
		Library:            lib,
		Activity:           act,
		YtdlpPath:          a.GetYtdlpPath,
		YtdlpVersion:       getYtdlpVersion,
		CookiesPath:        a.CookiesPath,
		PreferPremiumAudio: a.PreferPremiumAudio,
		DownloadDir:        a.DownloadDir,
		AutoDownload:       a.AutoDownloadToLibrary,
		ThumbDir:           filepath.Join(dataDir, "thumbs"),
		Bridge:             Version,
		AudioFormat:        cfg.AudioFormat,
		State:              holder,
		Emitter: events.EmitterFunc(func(ctx context.Context, name string, args ...any) {
			if ctx == nil {
				return
			}
			wailsRuntime.EventsEmit(ctx, name, args...)
		}),
	}

	br := bridge.New(holder, func() *http.Server {
		return &http.Server{
			Handler:           server.WithCORS(handlers.Router(), func() []string { return a.GetConfig().AllowedOrigins }),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      10 * time.Minute,
			IdleTimeout:       60 * time.Second,
		}
	})

	if cfg.ServerEnabled {
		startErr := br.Start(cfg.ListenPort)
		if startErr != nil && cfg.UseRandomIfBusy {
			slog.Warn("preferred port unavailable, retrying ephemeral", "preferred", cfg.ListenPort, "err", startErr)
			startErr = br.Start(0)
		}
		if startErr != nil {
			slog.Error("bridge start failed", "err", startErr)
		} else {
			slog.Info("bridge listening", "url", fmt.Sprintf("http://localhost:%d", br.Port()))
			if err := server.WritePortFile(dataDir, br.Port()); err != nil {
				slog.Warn("write port.txt failed", "err", err)
			}
		}
	}

	// COMPOSER_BRIDGE_MANIFEST_URL lets local testing point both the daily
	// poll and the Settings "Check now" button at a fake manifest server
	// without rebuilding the binary. Empty falls back to the production URL.
	manifestURL := os.Getenv("COMPOSER_BRIDGE_MANIFEST_URL")
	if manifestURL == "" {
		manifestURL = updater.DefaultManifestURL
	}
	a.SetManifestURL(manifestURL)

	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	scheduleYtdlpRefresh(bgCtx, dataDir,
		func() string { return a.GetConfig().YtdlpChannel },
		func() string { return a.GetConfig().YtdlpBinaryPath },
		refreshYtdlpVersion,
	)
	go updater.PollDaily(bgCtx, manifestURL, Version, func(info updater.UpdateInfo) {
		slog.Info("bridge update available", "version", info.Latest, "current", info.Current)
		a.SetLatestUpdate(&info)
		holder.SetUpdatePending(true)
		if appCtx := a.Ctx(); appCtx != nil {
			wailsRuntime.EventsEmit(appCtx, "bridge:update-available", info)
		}
	})

	a.SetYtdlpVersionFn(getYtdlpVersion)
	a.SetYtdlpVersionRefresher(refreshYtdlpVersion)
	a.SetYtdlpRefresher(func() {
		cfg := a.GetConfig()
		if cfg.YtdlpBinaryPath != "" {
			return
		}
		ytdlp.RefreshOnce(bgCtx, dataDir, cfg.YtdlpChannel, refreshYtdlpVersion)
	})
	a.SetBridgeState(holder)
	a.SetBridge(br)
	a.SetStatusEmitter(func(ctx context.Context, name string, data any) {
		if ctx == nil {
			return
		}
		wailsRuntime.EventsEmit(ctx, name, data)
	})

	if exec, err := os.Executable(); err == nil {
		if err := autostart.Refresh(exec); err != nil {
			slog.Warn("autostart refresh failed", "err", err)
		}
	}

	trayCtrl := tray.New()
	trayCtrl.SetState(holder)
	trayCtrl.SetOnStartServer(a.StartServer)
	trayCtrl.SetOnStopServer(a.StopServer)
	trayCtrl.SetOnCheckForUpdates(func() error {
		_, err := a.CheckForUpdates()
		return err
	})
	trayCtrl.SetRecentDownloads(func() []tray.RecentEntry {
		entries, err := act.Recent(5)
		if err != nil {
			slog.Warn("tray recent downloads failed", "err", err)
			return nil
		}
		out := make([]tray.RecentEntry, 0, len(entries))
		for _, e := range entries {
			if e.Kind != activity.KindAudioDownload {
				continue
			}
			entry := tray.RecentEntry{VideoID: e.VideoID}
			if track, err := lib.GetTrack(e.VideoID); err == nil && track != nil {
				title := track.Title
				if track.Artist != "" && title != "" {
					title = track.Artist + " - " + title
				}
				entry.Title = title
			}
			out = append(out, entry)
		}
		return out
	})
	trayCtrl.Register()

	err = wails.Run(&options.App{
		Title:     "Composer Bridge",
		Width:     1024,
		Height:    700,
		MinWidth:  800,
		MinHeight: 540,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: assetserver.ChainMiddleware(platformInjectMiddleware),
		},
		// A: 0 (transparent window background) is required for AppKit to draw
		// the rounded top corners on a `fullSizeContentView + titlebarAppearsTransparent`
		// window. The webview is transparent too, so the React app's
		// `bg-composer-bg` paints the visible interior. See wails issue #1805.
		BackgroundColour: &options.RGBA{R: 0x28, G: 0x29, B: 0x2c, A: 0},
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
			if err := br.Stop(); err != nil {
				slog.Warn("bridge stop failed", "err", err)
			}
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

// installGoroutineDumpSignal wires SIGQUIT (kill -QUIT <pid>) to write a full
// goroutine stack dump into dataDir/goroutines-<unix>.dump. The Go default
// handler for SIGQUIT prints stacks to stderr then exits; for a launchd-managed
// bridge stderr goes nowhere and exit kills the app. Routing through a file in
// dataDir survives both, so a wedged handler can be diagnosed without
// restarting the bridge.
func installGoroutineDumpSignal(dataDir string) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGQUIT)
	go func() {
		for range ch {
			path := filepath.Join(dataDir, fmt.Sprintf("goroutines-%d.dump", time.Now().Unix()))
			f, err := os.Create(path)
			if err != nil {
				slog.Warn("goroutine dump open failed", "path", path, "err", err)
				continue
			}
			if err := pprof.Lookup("goroutine").WriteTo(f, 2); err != nil {
				slog.Warn("goroutine dump write failed", "path", path, "err", err)
			}
			f.Close()
			slog.Info("wrote goroutine dump", "path", path)
		}
	}()
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
