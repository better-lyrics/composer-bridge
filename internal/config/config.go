package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Config is the on-disk shape of `~/.composer-bridge/config.json`. Every knob the user can tune lives here.
// Fields with an empty/zero value typically resolve to a runtime default; see Defaults and mergeDefaults for the rules.
type Config struct {
	ListenPort      int      `json:"listen_port"`
	UseRandomIfBusy bool     `json:"use_random_if_busy"`
	AllowedOrigins  []string `json:"allowed_origins"`
	YtdlpChannel    string   `json:"ytdlp_channel"`
	YtdlpBinaryPath string   `json:"ytdlp_binary_path"`
	OpenAtLogin     bool     `json:"open_at_login"`
	// ServerEnabled persists the user's last explicit choice toggled via
	// StartServer/StopServer. Fresh installs default to true via Defaults();
	// thereafter the on-disk value is authoritative so a tray-stopped state
	// survives restart.
	ServerEnabled bool `json:"server_enabled"`
	// CookiesEnabled gates whether yt-dlp receives the --cookies <path> flag.
	// The path on disk is always <dataDir>/cookies.txt; this boolean is what
	// the user toggles via Settings after uploading. Defaults to false on
	// fresh installs so a never-uploaded user gets the anonymous code path.
	CookiesEnabled bool `json:"cookies_enabled"`
	// PreferPremiumAudio asks yt-dlp to try YouTube Music's higher quality
	// tier first, gated on CookiesEnabled. Off by default because the probe
	// can add ~30s per request when Premium isn't actually available.
	PreferPremiumAudio bool   `json:"prefer_premium_audio"`
	ShowMenuBarIcon    bool   `json:"show_menu_bar_icon"`
	MaxConcurrent      int    `json:"max_concurrent"`
	AudioFormat        string `json:"audio_format"`
	AudioQuality       string `json:"audio_quality"`
	LogLevel           string `json:"log_level"`
	DataDir            string `json:"data_dir"`
	DownloadDir        string `json:"download_dir"`
	// AutoDownloadToLibrary, when true, makes /audio/{id} on a cache miss tee
	// the yt-dlp stdout into a file under DownloadDir while sending the same
	// bytes to the HTTP response. The next play for that videoID hits the
	// cache-first branch, so one yt-dlp invocation populates both. Off by
	// default so fresh installs match the documented "opt-in downloads" model.
	AutoDownloadToLibrary bool `json:"auto_download_to_library"`
}

// Defaults returns the canonical default Config. Each call returns a fresh value: mutating the result, including
// AllowedOrigins, never leaks into subsequent callers.
func Defaults() Config {
	return Config{
		ListenPort:      7777,
		UseRandomIfBusy: true,
		AllowedOrigins: []string{
			"https://composer.boidu.dev",
			"https://composer.betterlyrics.org",
			"http://localhost:5173",
			"http://localhost:5174",
			"http://localhost:5175",
			"http://localhost:4173",
			"http://127.0.0.1:5173",
			"http://127.0.0.1:5174",
		},
		YtdlpChannel:    "stable",
		OpenAtLogin:     false,
		ServerEnabled:   true,
		ShowMenuBarIcon: true,
		MaxConcurrent:   3,
		AudioFormat:     "opus",
		AudioQuality:    "best",
		LogLevel:        "info",
	}
}

// Load reads and decodes the config at path. A missing file returns Defaults() with a nil error.
// On read or parse failures, Load returns full Defaults() alongside a wrapped error: callers may use the
// returned Config as a safe fallback even when err is non-nil. Successful loads have unspecified fields
// backfilled by mergeDefaults so callers never see zero values for required knobs. If sanitization (origin
// splitting) changed the parsed AllowedOrigins, the cleaned shape is written back so the on-disk file
// migrates forward without a user-initiated save.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Defaults(), fmt.Errorf("read config: %w", err)
	}
	cfg := Defaults()
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Defaults(), fmt.Errorf("parse config: %w", err)
	}
	originalOrigins := append([]string(nil), cfg.AllowedOrigins...)
	cfg = mergeDefaults(cfg)
	if !slices.Equal(originalOrigins, cfg.AllowedOrigins) {
		if err := Save(path, cfg); err != nil {
			slog.Warn("rewrite cleaned config after origin migration", "err", err, "path", path)
		}
	}
	return cfg, nil
}

// Save serialises cfg as indented JSON to path, creating any missing parent
// directories. Writes are atomic: the bytes go to a unique tmp file
// (os.CreateTemp wildcard suffix) and are then renamed into place. The
// unique suffix matters because multiple Wails-bound callers (SaveConfig,
// UploadCookies, persistServerEnabled, etc.) hit Save concurrently; a fixed
// tmp filename let one writer truncate or rename-away another's in-flight
// file.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create config tmp: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return fmt.Errorf("write config tmp: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return fmt.Errorf("chmod config tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close config tmp: %w", err)
	}
	// TODO(windows): os.Rename onto an existing file fails on Windows; swap to a Windows-safe atomic rename helper if/when the bridge ships there.
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename config tmp: %w", err)
	}
	return nil
}

// SplitAndCleanOrigins re-splits any allowed-origin entry that has commas baked
// into it. The form input is a single comma-separated string, so the frontend
// may persist all origins as `allowed_origins[0]`. Loads call this at read
// time and Save callers call it at write time so the on-disk shape stays a
// clean array of separate origins. Idempotent: comma-free entries pass through.
func SplitAndCleanOrigins(origins []string) []string {
	if len(origins) == 0 {
		return origins
	}
	out := make([]string, 0, len(origins))
	for _, raw := range origins {
		for _, piece := range strings.Split(raw, ",") {
			piece = strings.TrimSpace(piece)
			if piece != "" {
				out = append(out, piece)
			}
		}
	}
	return out
}

func mergeDefaults(cfg Config) Config {
	d := Defaults()
	if cfg.ListenPort == 0 {
		cfg.ListenPort = d.ListenPort
	}
	cfg.AllowedOrigins = SplitAndCleanOrigins(cfg.AllowedOrigins)
	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = d.AllowedOrigins
	}
	if cfg.YtdlpChannel == "" {
		cfg.YtdlpChannel = d.YtdlpChannel
	}
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = d.MaxConcurrent
	}
	if cfg.AudioFormat == "" {
		cfg.AudioFormat = d.AudioFormat
	}
	if cfg.AudioQuality == "" {
		cfg.AudioQuality = d.AudioQuality
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = d.LogLevel
	}
	return cfg
}
