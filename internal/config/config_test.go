package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func tmpConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.json")
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestDefaults_ExposesDocumentedValues(t *testing.T) {
	d := Defaults()

	if d.ListenPort != 7777 {
		t.Errorf("ListenPort: got %d, want 7777", d.ListenPort)
	}
	if !d.UseRandomIfBusy {
		t.Errorf("UseRandomIfBusy: got false, want true")
	}
	wantOrigins := []string{
		"https://composer.boidu.dev",
		"https://composer.betterlyrics.org",
		"http://localhost:5173",
		"http://localhost:5174",
		"http://localhost:5175",
		"http://localhost:4173",
		"http://127.0.0.1:5173",
		"http://127.0.0.1:5174",
	}
	if !reflect.DeepEqual(d.AllowedOrigins, wantOrigins) {
		t.Errorf("AllowedOrigins: got %#v, want %#v", d.AllowedOrigins, wantOrigins)
	}
	if d.YtdlpChannel != "stable" {
		t.Errorf("YtdlpChannel: got %q, want %q", d.YtdlpChannel, "stable")
	}
	if d.YtdlpBinaryPath != "" {
		t.Errorf("YtdlpBinaryPath: got %q, want empty", d.YtdlpBinaryPath)
	}
	if d.OpenAtLogin {
		t.Errorf("OpenAtLogin: got true, want false")
	}
	if !d.ShowMenuBarIcon {
		t.Errorf("ShowMenuBarIcon: got false, want true")
	}
	if d.MaxConcurrent != 3 {
		t.Errorf("MaxConcurrent: got %d, want 3", d.MaxConcurrent)
	}
	if d.AudioFormat != "opus" {
		t.Errorf("AudioFormat: got %q, want %q", d.AudioFormat, "opus")
	}
	if d.AudioQuality != "best" {
		t.Errorf("AudioQuality: got %q, want %q", d.AudioQuality, "best")
	}
	if d.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", d.LogLevel, "info")
	}
	if d.DataDir != "" {
		t.Errorf("DataDir: got %q, want empty", d.DataDir)
	}
	if d.DownloadDir != "" {
		t.Errorf("DownloadDir: got %q, want empty", d.DownloadDir)
	}
}

func TestDefaults_AllowedOriginsIsNotAliased(t *testing.T) {
	a := Defaults()
	b := Defaults()
	if len(a.AllowedOrigins) == 0 {
		t.Fatal("AllowedOrigins: empty")
	}
	a.AllowedOrigins[0] = "https://mutated.example"
	if b.AllowedOrigins[0] == "https://mutated.example" {
		t.Errorf("AllowedOrigins: mutating one Defaults() result leaked into a second call; the slice must not be shared")
	}
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	path := tmpConfigPath(t)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(missing): %v", err)
	}
	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("Load(missing): got %#v, want defaults", cfg)
	}
}

func TestSaveLoad_RoundTripsAllFields(t *testing.T) {
	path := tmpConfigPath(t)

	in := Config{
		ListenPort:            9090,
		UseRandomIfBusy:       false,
		AllowedOrigins:        []string{"https://a.test", "https://b.test"},
		YtdlpChannel:          "nightly",
		YtdlpBinaryPath:       "/usr/local/bin/yt-dlp",
		OpenAtLogin:           true,
		ShowMenuBarIcon:       false,
		MaxConcurrent:         7,
		AudioFormat:           "opus",
		AudioQuality:          "192k",
		LogLevel:              "debug",
		DataDir:               "/var/lib/composer-bridge",
		DownloadDir:           "/home/me/Music",
		AutoDownloadToLibrary: true,
	}

	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch:\n in=%#v\nout=%#v", in, out)
	}
}

func TestSave_CreatesMissingParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "config.json")

	if err := Save(path, Defaults()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Save did not create file: %v", err)
	}
}

func TestLoad_PartialJSON_PortOnly_FillsRestFromDefaults(t *testing.T) {
	path := tmpConfigPath(t)
	writeFile(t, path, `{"listen_port": 8080}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenPort != 8080 {
		t.Errorf("ListenPort: got %d, want 8080", cfg.ListenPort)
	}

	d := Defaults()
	if !reflect.DeepEqual(cfg.AllowedOrigins, d.AllowedOrigins) {
		t.Errorf("AllowedOrigins: got %#v, want defaults %#v", cfg.AllowedOrigins, d.AllowedOrigins)
	}
	if cfg.YtdlpChannel != d.YtdlpChannel {
		t.Errorf("YtdlpChannel: got %q, want %q", cfg.YtdlpChannel, d.YtdlpChannel)
	}
	if cfg.MaxConcurrent != d.MaxConcurrent {
		t.Errorf("MaxConcurrent: got %d, want %d", cfg.MaxConcurrent, d.MaxConcurrent)
	}
	if cfg.AudioFormat != d.AudioFormat {
		t.Errorf("AudioFormat: got %q, want %q", cfg.AudioFormat, d.AudioFormat)
	}
	if cfg.AudioQuality != d.AudioQuality {
		t.Errorf("AudioQuality: got %q, want %q", cfg.AudioQuality, d.AudioQuality)
	}
	if cfg.LogLevel != d.LogLevel {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, d.LogLevel)
	}
}

func TestLoad_PartialJSON_SingletonOriginsListPreserved(t *testing.T) {
	path := tmpConfigPath(t)
	writeFile(t, path, `{"allowed_origins": ["https://only.test"]}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"https://only.test"}
	if !reflect.DeepEqual(cfg.AllowedOrigins, want) {
		t.Errorf("AllowedOrigins: got %#v, want %#v (a one-entry list is a real user choice, not 'unset')", cfg.AllowedOrigins, want)
	}
}

func TestLoad_PartialJSON_ZeroMaxConcurrentFallsBackToDefault(t *testing.T) {
	path := tmpConfigPath(t)
	writeFile(t, path, `{"max_concurrent": 0}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxConcurrent != 3 {
		t.Errorf("MaxConcurrent: got %d, want 3 (0 indistinguishable from absent, documented)", cfg.MaxConcurrent)
	}
}

func TestLoad_PartialJSON_FalseBoolPreserved(t *testing.T) {
	path := tmpConfigPath(t)
	writeFile(t, path, `{"open_at_login": false, "show_menu_bar_icon": false, "use_random_if_busy": false}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OpenAtLogin {
		t.Errorf("OpenAtLogin: got true, want false (bool fields must NOT be backfilled)")
	}
	if cfg.ShowMenuBarIcon {
		t.Errorf("ShowMenuBarIcon: got true, want false (bool fields must NOT be backfilled)")
	}
	if cfg.UseRandomIfBusy {
		t.Errorf("UseRandomIfBusy: got true, want false (bool fields must NOT be backfilled)")
	}
}

func TestLoad_PartialJSON_EmptyOriginsArrayTreatedAsAbsent(t *testing.T) {
	path := tmpConfigPath(t)
	writeFile(t, path, `{"listen_port": 8080, "allowed_origins": []}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenPort != 8080 {
		t.Errorf("ListenPort: got %d, want 8080", cfg.ListenPort)
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, Defaults().AllowedOrigins) {
		t.Errorf("AllowedOrigins: got %#v, want defaults (empty array is treated as absent on purpose)", cfg.AllowedOrigins)
	}
}

func TestLoad_CorruptJSONReturnsDefaultsAndError(t *testing.T) {
	path := tmpConfigPath(t)
	writeFile(t, path, `{not json`)

	cfg, err := Load(path)
	if err == nil {
		t.Fatal("Load(corrupt): want error, got nil")
	}
	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("Load(corrupt): cfg must be full defaults, got %#v", cfg)
	}
}

func TestLoad_LiteralNullReturnsDefaults(t *testing.T) {
	path := tmpConfigPath(t)
	writeFile(t, path, `null`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(null): %v (literal null must round-trip to defaults without error)", err)
	}
	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("Load(null): got %#v, want defaults", cfg)
	}
}

func TestLoad_ZeroByteFileReturnsDefaultsAndError(t *testing.T) {
	path := tmpConfigPath(t)
	writeFile(t, path, "")

	cfg, err := Load(path)
	if err == nil {
		t.Fatal("Load(zero-byte): want error, got nil")
	}
	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("Load(zero-byte): cfg must be full defaults, got %#v", cfg)
	}
}

func TestSaveLoad_UnicodePathRoundTrips(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "한국어-emoji-\U0001F3B5")
	path := filepath.Join(dir, "config.json")

	in := Defaults()
	in.ListenPort = 1234

	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch:\n in=%#v\nout=%#v", in, out)
	}
}

func TestSave_AtomicNoStaleTmpOnRenameFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rename-to-occupied semantics differ on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	subFile := filepath.Join(path, "occupant.txt")
	if err := os.WriteFile(subFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	err := Save(path, Defaults())
	if err == nil {
		t.Fatal("Save: want error (rename onto non-empty directory must fail), got nil")
	}

	if _, statErr := os.Stat(path + ".tmp"); !os.IsNotExist(statErr) {
		t.Errorf("Save left stale tmp file: stat err = %v", statErr)
	}
}

func TestSave_WritesIndentedJSON(t *testing.T) {
	path := tmpConfigPath(t)

	if err := Save(path, Defaults()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), "\n  \"") {
		t.Errorf("Save: file is not indented with two spaces:\n%s", raw)
	}
}

func TestSave_FileModeIsUserReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix mode bits not meaningful on Windows")
	}

	path := tmpConfigPath(t)
	if err := Save(path, Defaults()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	mode := info.Mode().Perm()
	if mode&0o400 == 0 {
		t.Errorf("FileMode: got %o, want at least user-readable (0o400)", mode)
	}
}

func TestSave_ExplicitFalseBoolRoundTripsViaJSON(t *testing.T) {
	path := tmpConfigPath(t)

	in := Defaults()
	in.UseRandomIfBusy = false
	in.ShowMenuBarIcon = false

	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v, ok := generic["use_random_if_busy"]; !ok || v != false {
		t.Errorf("use_random_if_busy: got %v (ok=%v), want explicit false key in JSON", v, ok)
	}
	if v, ok := generic["show_menu_bar_icon"]; !ok || v != false {
		t.Errorf("show_menu_bar_icon: got %v (ok=%v), want explicit false key in JSON", v, ok)
	}
}

func TestSaveLoad_DeepEqualRegressionGuard(t *testing.T) {
	path := tmpConfigPath(t)

	in := Config{
		ListenPort:            4242,
		UseRandomIfBusy:       false,
		AllowedOrigins:        []string{"https://x.test"},
		YtdlpChannel:          "nightly",
		YtdlpBinaryPath:       "/opt/ytdlp",
		OpenAtLogin:           true,
		ShowMenuBarIcon:       false,
		MaxConcurrent:         9,
		AudioFormat:           "mp3",
		AudioQuality:          "320k",
		LogLevel:              "warn",
		DataDir:               "/tmp/data",
		DownloadDir:           "/tmp/dl",
		AutoDownloadToLibrary: true,
	}

	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("DeepEqual round trip failed:\n in=%#v\nout=%#v", in, out)
	}
}

func TestDefaults_ServerEnabledIsTrue(t *testing.T) {
	d := Defaults()
	if !d.ServerEnabled {
		t.Errorf("ServerEnabled: got false, want true (server should be enabled by default)")
	}
}

func TestSave_NoStaleTmpOnSuccess(t *testing.T) {
	path := tmpConfigPath(t)

	if err := Save(path, Defaults()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	stale, err := filepath.Glob(path + ".*.tmp")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(stale) > 0 {
		t.Errorf("Save: stale tmp files present after successful save: %v", stale)
	}
}

func TestSave_Load_PreservesCookiesEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Defaults()
	cfg.CookiesEnabled = true
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.CookiesEnabled {
		t.Fatalf("CookiesEnabled lost on round-trip")
	}
}

func TestDefaults_CookiesEnabledFalse(t *testing.T) {
	if Defaults().CookiesEnabled {
		t.Fatalf("Defaults().CookiesEnabled should be false (no auto-flagged cookies on fresh install)")
	}
}

func TestSave_Load_PreservesPreferPremiumAudio(t *testing.T) {
	path := tmpConfigPath(t)
	cfg := Defaults()
	cfg.PreferPremiumAudio = true
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.PreferPremiumAudio {
		t.Fatalf("PreferPremiumAudio lost on round-trip")
	}
}

// TestSave_ConcurrentWritersDoNotCollideOnTmp guards the unique tmp suffix:
// without it, two goroutines racing through Save would write to the same
// path+".tmp" and one would lose its rename target. With os.CreateTemp's
// wildcard suffix each writer gets a unique tmp file, so all renames land
// and the final file is a valid config from one of the writers.
func TestSave_ConcurrentWritersDoNotCollideOnTmp(t *testing.T) {
	path := tmpConfigPath(t)
	const writers = 16

	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		cfg := Defaults()
		cfg.ListenPort = 8000 + i
		go func() { errs <- Save(path, cfg) }()
	}
	for i := 0; i < writers; i++ {
		if err := <-errs; err != nil {
			t.Errorf("Save #%d: %v", i, err)
		}
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after concurrent saves: %v", err)
	}
	if loaded.ListenPort < 8000 || loaded.ListenPort >= 8000+writers {
		t.Errorf("Load: ListenPort %d not from any concurrent writer", loaded.ListenPort)
	}
	stale, err := filepath.Glob(path + ".*.tmp")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(stale) > 0 {
		t.Errorf("stale tmp files left behind: %v", stale)
	}
}

func TestDefaults_PreferPremiumAudioFalse(t *testing.T) {
	if Defaults().PreferPremiumAudio {
		t.Fatalf("Defaults().PreferPremiumAudio should be false (opt-in)")
	}
}

func TestDefaults_AutoDownloadToLibraryFalse(t *testing.T) {
	if Defaults().AutoDownloadToLibrary {
		t.Fatalf("Defaults().AutoDownloadToLibrary should be false (opt-in)")
	}
}

func TestSave_Load_PreservesAutoDownloadToLibrary(t *testing.T) {
	path := tmpConfigPath(t)
	cfg := Defaults()
	cfg.AutoDownloadToLibrary = true
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.AutoDownloadToLibrary {
		t.Fatalf("AutoDownloadToLibrary lost on round-trip")
	}
}
