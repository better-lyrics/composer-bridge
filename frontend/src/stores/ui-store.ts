import { create } from "zustand";
import type { activity, bridgestate, config, library, updater } from "../../wailsjs/go/models";

export type View = "library" | "activity" | "settings";
export type LibrarySort = "recent" | "title" | "artist" | "duration";

interface UIState {
  view: View;
  setView: (view: View) => void;
  selectedVideoId: string | null;
  setSelectedVideoId: (id: string | null) => void;
  librarySort: LibrarySort;
  setLibrarySort: (sort: LibrarySort) => void;
  librarySearch: string;
  setLibrarySearch: (search: string) => void;
  activeDownloads: Set<string>;
  beginDownload: (videoID: string) => void;
  endDownload: (videoID: string) => void;
  libraryTracks: library.Track[] | null;
  setLibraryTracks: (tracks: library.Track[]) => void;
  bridgeConfig: config.Config | null;
  setBridgeConfig: (cfg: config.Config) => void;
  activityEntries: activity.Entry[] | null;
  setActivityEntries: (entries: activity.Entry[]) => void;
  bridgeStatus: bridgestate.State | null;
  setBridgeStatus: (status: bridgestate.State) => void;
  updateInfo: updater.UpdateInfo | null;
  setUpdateInfo: (info: updater.UpdateInfo | null) => void;
  updateBannerDismissed: boolean;
  setUpdateBannerDismissed: (dismissed: boolean) => void;
}

export const useUIStore = create<UIState>((set) => ({
  view: "library",
  setView: (view) => set({ view }),
  selectedVideoId: null,
  setSelectedVideoId: (selectedVideoId) => set({ selectedVideoId }),
  librarySort: "recent",
  setLibrarySort: (librarySort) => set({ librarySort }),
  librarySearch: "",
  setLibrarySearch: (librarySearch) => set({ librarySearch }),
  activeDownloads: new Set<string>(),
  beginDownload: (videoID) =>
    set((s) => {
      if (s.activeDownloads.has(videoID)) return s;
      const next = new Set(s.activeDownloads);
      next.add(videoID);
      return { activeDownloads: next };
    }),
  endDownload: (videoID) =>
    set((s) => {
      if (!s.activeDownloads.has(videoID)) return s;
      const next = new Set(s.activeDownloads);
      next.delete(videoID);
      return { activeDownloads: next };
    }),
  libraryTracks: null,
  setLibraryTracks: (libraryTracks) => set({ libraryTracks }),
  bridgeConfig: null,
  setBridgeConfig: (bridgeConfig) => set({ bridgeConfig }),
  activityEntries: null,
  setActivityEntries: (activityEntries) => set({ activityEntries }),
  bridgeStatus: null,
  setBridgeStatus: (bridgeStatus) => set({ bridgeStatus }),
  updateInfo: null,
  setUpdateInfo: (updateInfo) => set({ updateInfo }),
  updateBannerDismissed: false,
  setUpdateBannerDismissed: (updateBannerDismissed) => set({ updateBannerDismissed }),
}));
