import { create } from "zustand";

export type View = "library" | "activity" | "settings";

interface UIState {
  view: View;
  setView: (view: View) => void;
}

export const useUIStore = create<UIState>((set) => ({
  view: "library",
  setView: (view) => set({ view }),
}));
