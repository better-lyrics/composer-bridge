import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { LibraryView } from "@/views/library/library-view";
import { useUIStore } from "@/stores/ui-store";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";

function makeTrack(overrides: Record<string, unknown> = {}) {
  return {
    ID: 1,
    VideoID: "RgKAFK5djSk",
    Title: "Hey Jude",
    Artist: "The Beatles",
    Album: "",
    ReleaseYear: 0,
    DurationSec: 431,
    ThumbnailURL: "",
    ThumbPath: "",
    IsMusic: true,
    MusicType: "song",
    SourceURL: "",
    ImportedAt: 1000,
    AudioPath: "",
    AudioSize: 0,
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

  it("empty state CTA copies the import URL to clipboard", async () => {
    render(<LibraryView />);
    await waitFor(() =>
      expect(screen.getByText(/No tracks yet/i)).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: /Copy/i }));
    await waitFor(() =>
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        "http://localhost:7777/import",
      ),
    );
  });

  it("renders a card for every track with title, artist, and duration", async () => {
    bindings.ListTracks.mockResolvedValue([
      makeTrack({ VideoID: "aaaaaaaaaaa", Title: "Alpha Song", Artist: "Alice", DurationSec: 60 }),
      makeTrack({ VideoID: "bbbbbbbbbbb", Title: "Beta Tune", Artist: "Bob", DurationSec: 125 }),
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
      makeTrack({ VideoID: "aaaaaaaaaaa", Title: "Alpha" }),
    ]);
    render(<LibraryView />);
    await waitFor(() => expect(screen.getByText("Alpha")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Alpha"));
    expect(useUIStore.getState().selectedVideoId).toBe("aaaaaaaaaaa");
  });

  it("search filters cards by title case-insensitively", async () => {
    bindings.ListTracks.mockResolvedValue([
      makeTrack({ VideoID: "aaaaaaaaaaa", Title: "Sunrise" }),
      makeTrack({ VideoID: "bbbbbbbbbbb", Title: "Moonlight" }),
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
      makeTrack({ VideoID: "aaaaaaaaaaa", Title: "Zebra", ImportedAt: 2000 }),
      makeTrack({ VideoID: "bbbbbbbbbbb", Title: "Apple", ImportedAt: 1000 }),
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
