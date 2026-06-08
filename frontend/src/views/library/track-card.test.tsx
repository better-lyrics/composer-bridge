import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TrackCard } from "@/views/library/track-card";

function makeTrack(overrides: Record<string, unknown> = {}) {
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
  } as unknown as Parameters<typeof TrackCard>[0]["track"];
}

describe("TrackCard", () => {
  it("music track renders with square aspect", () => {
    render(<TrackCard track={makeTrack({ is_music: true })} onSelect={vi.fn()} />);
    const wrapper = screen
      .getByRole("button")
      .querySelector(".aspect-square");
    expect(wrapper).not.toBeNull();
  });

  it("video track renders with 16:9 aspect", () => {
    render(<TrackCard track={makeTrack({ is_music: false })} onSelect={vi.fn()} />);
    const wrapper = screen
      .getByRole("button")
      .querySelector(".aspect-video");
    expect(wrapper).not.toBeNull();
  });

  it("metadata-only track shows Metadata only badge", () => {
    render(<TrackCard track={makeTrack({ audio_path: "" })} onSelect={vi.fn()} />);
    expect(screen.getByText(/Metadata only/i)).toBeInTheDocument();
  });

  it("downloaded track shows Downloaded badge in bl-red", () => {
    render(
      <TrackCard
        track={makeTrack({ audio_path: "/tmp/audio.m4a" })}
        onSelect={vi.fn()}
      />,
    );
    const badge = screen.getByText(/Downloaded/i);
    expect(badge.className).toMatch(/bg-bl-red/);
  });
});
