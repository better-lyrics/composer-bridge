import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { LibraryView } from "@/views/library/library-view";
import { useUIStore } from "@/stores/ui-store";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";

function makeTrack(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    video_id: "RgKAFK5djSk",
    title: "Hey Jude",
    artist: "The Beatles",
    album: "",
    release_year: 0,
    duration_sec: 431,
    thumbnail_url: "",
    thumb_path: "",
    is_music: true,
    music_type: "song",
    source_url: "",
    imported_at: 1000,
    audio_path: "",
    audio_size: 0,
    ...overrides,
  };
}

let bindings: AppBindings;

beforeEach(() => {
  useUIStore.setState({
    selectedVideoId: null,
    librarySort: "recent",
    librarySearch: "",
  });
  bindings = setupWailsMock();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});

afterEach(() => {
  cleanup();
  resetWailsMock();
});

describe("LibraryView", () => {
  it("renders empty state when no tracks", async () => {
    render(<LibraryView />);
    await waitFor(() => {
      expect(screen.getByText(/No tracks yet/i)).toBeInTheDocument();
    });
  });

  it("empty state CTA opens Composer via the Wails runtime", async () => {
    render(<LibraryView />);
    await waitFor(() =>
      expect(screen.getByText(/No tracks yet/i)).toBeInTheDocument(),
    );
    const runtime = (window as unknown as { runtime: { BrowserOpenURL: ReturnType<typeof vi.fn> } })
      .runtime;
    fireEvent.click(screen.getByRole("button", { name: /Open Composer/i }));
    await waitFor(() =>
      expect(runtime.BrowserOpenURL).toHaveBeenCalledWith("https://composer.boidu.dev"),
    );
  });

  it("renders a card for every track with title, artist, and duration", async () => {
    bindings.ListTracks.mockResolvedValue([
      makeTrack({ video_id: "aaaaaaaaaaa", title: "Alpha Song", artist: "Alice", duration_sec: 60 }),
      makeTrack({ video_id: "bbbbbbbbbbb", title: "Beta Tune", artist: "Bob", duration_sec: 125 }),
    ]);
    render(<LibraryView />);
    await waitFor(() => {
      expect(screen.getByText("Alpha Song")).toBeInTheDocument();
      expect(screen.getByText("Beta Tune")).toBeInTheDocument();
    });
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Bob")).toBeInTheDocument();
    expect(screen.getByText("1:00")).toBeInTheDocument();
    expect(screen.getByText("2:05")).toBeInTheDocument();
  });

  it("clicking a card sets selectedVideoId in the store", async () => {
    bindings.ListTracks.mockResolvedValue([
      makeTrack({ video_id: "aaaaaaaaaaa", title: "Alpha" }),
    ]);
    render(<LibraryView />);
    await waitFor(() => expect(screen.getByText("Alpha")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Alpha"));
    expect(useUIStore.getState().selectedVideoId).toBe("aaaaaaaaaaa");
  });

  it("search filters cards by title case-insensitively", async () => {
    bindings.ListTracks.mockResolvedValue([
      makeTrack({ video_id: "aaaaaaaaaaa", title: "Sunrise" }),
      makeTrack({ video_id: "bbbbbbbbbbb", title: "Moonlight" }),
    ]);
    render(<LibraryView />);
    await waitFor(() => expect(screen.getByText("Sunrise")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText(/Search library/i), {
      target: { value: "MOON" },
    });
    await waitFor(() => {
      expect(screen.queryByText("Sunrise")).not.toBeInTheDocument();
      expect(screen.getByText("Moonlight")).toBeInTheDocument();
    });
  });

  it("sort by title A-Z reorders the rendered cards", async () => {
    bindings.ListTracks.mockResolvedValue([
      makeTrack({ video_id: "aaaaaaaaaaa", title: "Zebra", imported_at: 2000 }),
      makeTrack({ video_id: "bbbbbbbbbbb", title: "Apple", imported_at: 1000 }),
    ]);
    render(<LibraryView />);
    await waitFor(() => expect(screen.getByText("Apple")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText(/Sort tracks/i), {
      target: { value: "title" },
    });
    await waitFor(() => {
      const titles = screen
        .getAllByTestId("track-card")
        .map((card) => card.querySelector("span")?.textContent);
      expect(titles).toEqual(["Apple", "Zebra"]);
    });
  });
});
