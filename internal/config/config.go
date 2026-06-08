package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	ShowMenuBarIcon bool     `json:"show_menu_bar_icon"`
	MaxConcurrent   int      `json:"max_concurrent"`
	AudioFormat     string   `json:"audio_format"`
	AudioQuality    string   `json:"audio_quality"`
	LogLevel        string   `json:"log_level"`
	DataDir         string   `json:"data_dir"`
	DownloadDir     string   `json:"download_dir"`
}

// Defaults returns the canonical default Config. Each call returns a fresh value: mutating the result, including
// AllowedOrigins, never leaks into subsequent callers.
func Defaults() Config {
	return Config{
		ListenPort:      7777,
		UseRandomIfBusy: true,
		AllowedOrigins: []string{
			"https://composer.boidu.dev",
			"https://composer-staging.boidu.dev",
			"http://localhost:5173",
			"http://localhost:5174",
			"http://localhost:5175",
			"http://localhost:4173",
			"http://127.0.0.1:5173",
			"http://127.0.0.1:5174",
		},
		YtdlpChannel:    "stable",
		OpenAtLogin:     false,
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
// backfilled by mergeDefaults so callers never see zero values for required knobs.
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
	return mergeDefaults(cfg), nil
}

// Save serialises cfg as indented JSON to path, creating any missing parent directories. Writes are atomic:
// the bytes go to path+".tmp" first and are then renamed into place; on rename failure the temp file is removed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write config tmp: %w", err)
	}
	// TODO(windows): os.Rename onto an existing file fails on Windows; swap to a Windows-safe atomic rename helper if/when the bridge ships there.
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename config tmp: %w", err)
	}
	return nil
}

func mergeDefaults(cfg Config) Config {
	d := Defaults()
	if cfg.ListenPort == 0 {
		cfg.ListenPort = d.ListenPort
	}
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
