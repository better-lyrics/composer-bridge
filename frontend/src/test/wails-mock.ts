import { vi, type Mock } from "vitest";
import type { activity, bridgestate, config, library } from "../../wailsjs/go/models";

// AppBindings mirrors the auto-generated wailsjs/go/app/App.d.ts surface.
// All fields are vi.fn() so tests can assert call shape and override returns.
export interface AppBindings {
  ListTracks: Mock<() => Promise<library.Track[]>>;
  GetTrack: Mock<(videoID: string) => Promise<library.Track | null>>;
  RemoveTrack: Mock<(videoID: string) => Promise<void>>;
  RecentActivity: Mock<(limit: number) => Promise<activity.Entry[]>>;
  GetConfig: Mock<() => Promise<config.Config>>;
  SaveConfig: Mock<(cfg: config.Config) => Promise<void>>;
  OpenInComposer: Mock<(videoID: string) => Promise<string>>;
  OpenInYouTube: Mock<(videoID: string) => Promise<string>>;
  BridgeVersion: Mock<() => Promise<string>>;
  YtdlpVersion: Mock<() => Promise<string>>;
  LibrarySize: Mock<() => Promise<number>>;
  ThumbCacheSize: Mock<() => Promise<number>>;
  ForceYtdlpUpdate: Mock<() => Promise<string>>;
  DownloadAudio: Mock<(videoID: string) => Promise<library.Track>>;
  OpenLogFile: Mock<() => Promise<string>>;
  BuildDiagnosticReport: Mock<() => Promise<string>>;
  SupportsAutostart: Mock<() => Promise<boolean>>;
  BridgeStatus: Mock<() => Promise<bridgestate.State>>;
  StartServer: Mock<() => Promise<void>>;
  StopServer: Mock<() => Promise<void>>;
}

const DEFAULT_CONFIG = {
  listen_port: 7777,
  use_random_if_busy: true,
  allowed_origins: ["https://composer.boidu.dev"],
  ytdlp_channel: "stable",
  ytdlp_binary_path: "",
  open_at_login: false,
  show_menu_bar_icon: true,
  max_concurrent: 3,
  audio_format: "opus",
  audio_quality: "best",
  log_level: "info",
  data_dir: "",
  download_dir: "",
} as unknown as config.Config;

// setupWailsMock installs a vi-mocked window.go.app.App surface and a window.runtime
// shim. Anything not in `overrides` resolves to a sensible empty default so tests
// don't accidentally read undefined.
export function setupWailsMock(overrides: Partial<AppBindings> = {}): AppBindings {
  const emptyTrack: library.Track = {
    id: 0,
    video_id: "",
    title: "",
    artist: "",
    album: "",
    release_year: 0,
    duration_sec: 0,
    thumbnail_url: "",
    thumb_path: "",
    is_music: false,
    music_type: "",
    source_url: "",
    imported_at: 0,
    audio_path: "",
    audio_size: 0,
  };
  const bindings: AppBindings = {
    ListTracks: vi.fn<() => Promise<library.Track[]>>().mockResolvedValue([]),
    GetTrack: vi.fn<(videoID: string) => Promise<library.Track | null>>().mockResolvedValue(null),
    RemoveTrack: vi.fn<(videoID: string) => Promise<void>>().mockResolvedValue(undefined),
    RecentActivity: vi.fn<(limit: number) => Promise<activity.Entry[]>>().mockResolvedValue([]),
    GetConfig: vi.fn<() => Promise<config.Config>>().mockResolvedValue({ ...DEFAULT_CONFIG }),
    SaveConfig: vi.fn<(cfg: config.Config) => Promise<void>>().mockResolvedValue(undefined),
    OpenInComposer: vi
      .fn<(videoID: string) => Promise<string>>()
      .mockImplementation((id: string) => Promise.resolve(`https://composer.boidu.dev/?yt=${id}`)),
    OpenInYouTube: vi
      .fn<(videoID: string) => Promise<string>>()
      .mockImplementation((id: string) => Promise.resolve(`https://www.youtube.com/watch?v=${id}`)),
    BridgeVersion: vi.fn<() => Promise<string>>().mockResolvedValue("0.1.0-test"),
    YtdlpVersion: vi.fn<() => Promise<string>>().mockResolvedValue("2026.01.01"),
    LibrarySize: vi.fn<() => Promise<number>>().mockResolvedValue(0),
    ThumbCacheSize: vi.fn<() => Promise<number>>().mockResolvedValue(0),
    ForceYtdlpUpdate: vi.fn<() => Promise<string>>().mockResolvedValue("2026.01.02"),
    DownloadAudio: vi
      .fn<(videoID: string) => Promise<library.Track>>()
      .mockResolvedValue(emptyTrack),
    OpenLogFile: vi.fn<() => Promise<string>>().mockResolvedValue("file:///tmp/bridge.log"),
    BuildDiagnosticReport: vi.fn<() => Promise<string>>().mockResolvedValue("diagnostics"),
    SupportsAutostart: vi.fn<() => Promise<boolean>>().mockResolvedValue(true),
    BridgeStatus: vi.fn<() => Promise<bridgestate.State>>().mockResolvedValue({
      server: "stopped",
      download: "idle",
      downloadVideoId: "",
      lastError: "",
    } as bridgestate.State),
    StartServer: vi.fn<() => Promise<void>>().mockResolvedValue(undefined),
    StopServer: vi.fn<() => Promise<void>>().mockResolvedValue(undefined),
    ...overrides,
  };
  (window as unknown as { go: { app: { App: AppBindings } } }).go = {
    app: { App: bindings },
  };
  const runtime: WailsRuntime = {
    EventsOn: vi.fn(),
    EventsOnMultiple: vi.fn(),
    EventsEmit: vi.fn(),
    EventsOff: vi.fn(),
    BrowserOpenURL: vi.fn(),
  };
  (window as unknown as { runtime: WailsRuntime }).runtime = runtime;
  return bindings;
}

// WailsRuntime exposes the vi-mocked event bus that EventsOn/EventsOff route through.
// Tests that need to assert subscription shape or invoke a captured handler can grab it
// via getWailsRuntime().
export interface WailsRuntime {
  EventsOn: Mock;
  EventsOnMultiple: Mock;
  EventsEmit: Mock;
  EventsOff: Mock;
  BrowserOpenURL: Mock;
}

export function getWailsRuntime(): WailsRuntime {
  const runtime = (window as unknown as { runtime?: WailsRuntime }).runtime;
  if (!runtime) {
    throw new Error("Wails runtime mock not installed. Call setupWailsMock() first.");
  }
  return runtime;
}

// resetWailsMock clears any installed bindings between tests so leftover state
// from one test never bleeds into the next.
export function resetWailsMock(): void {
  delete (window as unknown as { go?: unknown }).go;
  delete (window as unknown as { runtime?: unknown }).runtime;
}
