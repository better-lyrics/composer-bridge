import { create } from "zustand";

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
}));
