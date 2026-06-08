import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { DetailPanel } from "@/views/library/detail-panel";
import { useUIStore } from "@/stores/ui-store";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";

let bindings: AppBindings;

function track(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    video_id: "RgKAFK5djSk",
    title: "Test",
    artist: "Artist",
    album: "",
    release_year: 0,
    duration_sec: 200,
    thumbnail_url: "",
    thumb_path: "",
    is_music: true,
    music_type: "song",
    source_url: "https://youtube.com/watch?v=RgKAFK5djSk",
    imported_at: 1,
    audio_path: "",
    audio_size: 0,
    ...overrides,
  };
}

beforeEach(() => {
  useUIStore.setState({ selectedVideoId: "RgKAFK5djSk" });
  bindings = setupWailsMock({
    GetTrack: vi.fn().mockResolvedValue(track()),
  });
});

afterEach(() => {
  cleanup();
  resetWailsMock();
  useUIStore.setState({ selectedVideoId: null });
});

describe("DetailPanel", () => {
  it("clicking Open in Composer calls the Wails binding", async () => {
    render(<DetailPanel onRemoved={vi.fn()} />);
    const openBtn = await screen.findByRole("button", { name: /Open in Composer/i });
    fireEvent.click(openBtn);
    await waitFor(() => expect(bindings.OpenInComposer).toHaveBeenCalledWith("RgKAFK5djSk"));
  });

  it("clicking Open in YouTube calls the Wails binding", async () => {
    render(<DetailPanel onRemoved={vi.fn()} />);
    const openBtn = await screen.findByRole("button", { name: /Open in YouTube/i });
    fireEvent.click(openBtn);
    await waitFor(() => expect(bindings.OpenInYouTube).toHaveBeenCalledWith("RgKAFK5djSk"));
  });

  it("Remove flow shows confirm dialog; confirming calls RemoveTrack and reloads", async () => {
    const onRemoved = vi.fn();
    render(<DetailPanel onRemoved={onRemoved} />);
    const removeBtn = await screen.findByRole("button", { name: /Remove from library/i });
    fireEvent.click(removeBtn);
    expect(screen.getByRole("dialog", { name: /Remove this track\?/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^Remove$/i }));
    await waitFor(() => expect(bindings.RemoveTrack).toHaveBeenCalledWith("RgKAFK5djSk"));
    expect(useUIStore.getState().selectedVideoId).toBeNull();
    expect(onRemoved).toHaveBeenCalled();
  });

  it("Cancel from confirm dialog leaves RemoveTrack untouched", async () => {
    render(<DetailPanel onRemoved={vi.fn()} />);
    const removeBtn = await screen.findByRole("button", { name: /Remove from library/i });
    fireEvent.click(removeBtn);
    fireEvent.click(screen.getByRole("button", { name: /Cancel/i }));
    expect(bindings.RemoveTrack).not.toHaveBeenCalled();
    expect(useUIStore.getState().selectedVideoId).toBe("RgKAFK5djSk");
  });
});
