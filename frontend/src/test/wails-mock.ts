import { vi, type Mock } from "vitest";
import type { library, activity, config } from "../../wailsjs/go/models";

// AppBindings mirrors the auto-generated wailsjs/go/main/App.d.ts surface.
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
  audio_format: "m4a",
  audio_quality: "best",
  log_level: "info",
  data_dir: "",
  download_dir: "",
} as unknown as config.Config;

// setupWailsMock installs a vi-mocked window.go.main.App surface and a window.runtime
// shim. Anything not in `overrides` resolves to a sensible empty default so tests
// don't accidentally read undefined.
export function setupWailsMock(overrides: Partial<AppBindings> = {}): AppBindings {
  const bindings: AppBindings = {
    ListTracks: vi.fn().mockResolvedValue([]),
    GetTrack: vi.fn().mockResolvedValue(null),
    RemoveTrack: vi.fn().mockResolvedValue(undefined),
    RecentActivity: vi.fn().mockResolvedValue([]),
    GetConfig: vi.fn().mockResolvedValue({ ...DEFAULT_CONFIG }),
    SaveConfig: vi.fn().mockResolvedValue(undefined),
    OpenInComposer: vi.fn().mockImplementation((id: string) =>
      Promise.resolve(`https://composer.boidu.dev/?yt=${id}`),
    ),
    OpenInYouTube: vi.fn().mockImplementation((id: string) =>
      Promise.resolve(`https://www.youtube.com/watch?v=${id}`),
    ),
    BridgeVersion: vi.fn().mockResolvedValue("0.1.0-test"),
    ...overrides,
  };
  (window as unknown as { go: { main: { App: AppBindings } } }).go = {
    main: { App: bindings },
  };
  type EventBus = {
    EventsOn: Mock;
    EventsOnMultiple: Mock;
    EventsEmit: Mock;
    EventsOff: Mock;
    BrowserOpenURL: Mock;
  };
  const runtime: EventBus = {
    EventsOn: vi.fn(),
    EventsOnMultiple: vi.fn(),
    EventsEmit: vi.fn(),
    EventsOff: vi.fn(),
    BrowserOpenURL: vi.fn(),
  };
  (window as unknown as { runtime: EventBus }).runtime = runtime;
  return bindings;
}

// resetWailsMock clears any installed bindings between tests so leftover state
// from one test never bleeds into the next.
export function resetWailsMock(): void {
  delete (window as unknown as { go?: unknown }).go;
  delete (window as unknown as { runtime?: unknown }).runtime;
}
