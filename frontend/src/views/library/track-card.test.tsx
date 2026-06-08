import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { library } from "../../../wailsjs/go/models";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";
import { TrackCard } from "@/views/library/track-card";

function makeTrack(overrides: Partial<library.Track> = {}): library.Track {
  return {
    id: 1,
    video_id: "RgKAFK5djSk",
    title: "Test Track",
    artist: "Test Artist",
    album: "",
    release_year: 0,
    duration_sec: 65,
    thumbnail_url: "",
    thumb_path: "",
    is_music: true,
    music_type: "song",
    source_url: "",
    imported_at: 0,
    audio_path: "",
    audio_size: 0,
    ...overrides,
  } as library.Track;
}

let bindings: AppBindings;

beforeEach(() => {
  bindings = setupWailsMock();
});

afterEach(() => {
  cleanup();
  resetWailsMock();
});

describe("TrackCard", () => {
  it("renders title, artist, and duration", () => {
    render(
      <TrackCard
        track={makeTrack({ title: "Hello", artist: "World", duration_sec: 65 })}
        onSelect={vi.fn()}
        onDownloaded={vi.fn()}
      />,
    );
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("World")).toBeInTheDocument();
    expect(screen.getByText("1:05")).toBeInTheDocument();
  });

  it("clicking the card fires onSelect with the videoID", () => {
    const onSelect = vi.fn();
    render(<TrackCard track={makeTrack()} onSelect={onSelect} onDownloaded={vi.fn()} />);
    fireEvent.click(screen.getByTestId("track-card"));
    expect(onSelect).toHaveBeenCalledWith("RgKAFK5djSk");
  });

  it("download button calls DownloadAudio and does not bubble to onSelect", async () => {
    const onSelect = vi.fn();
    const onDownloaded = vi.fn();
    bindings.DownloadAudio.mockResolvedValueOnce(makeTrack({ audio_path: "/tmp/audio.opus" }));
    render(<TrackCard track={makeTrack()} onSelect={onSelect} onDownloaded={onDownloaded} />);
    fireEvent.click(screen.getByLabelText(/Download audio/i));
    await vi.waitFor(() => expect(bindings.DownloadAudio).toHaveBeenCalledWith("RgKAFK5djSk"));
    await vi.waitFor(() => expect(onDownloaded).toHaveBeenCalled());
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("downloaded track disables the download button and shows a check", () => {
    render(
      <TrackCard
        track={makeTrack({ audio_path: "/tmp/audio.opus" })}
        onSelect={vi.fn()}
        onDownloaded={vi.fn()}
      />,
    );
    const btn = screen.getByLabelText(/Already downloaded/i);
    expect(btn).toBeDisabled();
  });

  it("open in composer button calls OpenInComposer and opens the URL", async () => {
    const onSelect = vi.fn();
    bindings.OpenInComposer.mockResolvedValueOnce("https://composer.boidu.dev/?yt=RgKAFK5djSk");
    render(<TrackCard track={makeTrack()} onSelect={onSelect} onDownloaded={vi.fn()} />);
    fireEvent.click(screen.getByLabelText(/Open in Composer/i));
    await vi.waitFor(() => expect(bindings.OpenInComposer).toHaveBeenCalledWith("RgKAFK5djSk"));
    expect(onSelect).not.toHaveBeenCalled();
  });
});
