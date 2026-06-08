import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TrackCard } from "@/views/library/track-card";

function makeTrack(overrides: Record<string, unknown> = {}) {
  return {
    ID: 1,
    VideoID: "RgKAFK5djSk",
    Title: "Test Track",
    Artist: "Test Artist",
    Album: "",
    ReleaseYear: 0,
    DurationSec: 65,
    ThumbnailURL: "",
    ThumbPath: "",
    IsMusic: true,
    MusicType: "song",
    SourceURL: "",
    ImportedAt: 0,
    AudioPath: "",
    AudioSize: 0,
    ...overrides,
  } as unknown as Parameters<typeof TrackCard>[0]["track"];
}

describe("TrackCard", () => {
  it("music track renders with square aspect", () => {
    render(<TrackCard track={makeTrack({ IsMusic: true })} onSelect={vi.fn()} />);
    const wrapper = screen
      .getByRole("button")
      .querySelector(".aspect-square");
    expect(wrapper).not.toBeNull();
  });

  it("video track renders with 16:9 aspect", () => {
    render(<TrackCard track={makeTrack({ IsMusic: false })} onSelect={vi.fn()} />);
    const wrapper = screen
      .getByRole("button")
      .querySelector(".aspect-video");
    expect(wrapper).not.toBeNull();
  });

  it("metadata-only track shows Metadata only badge", () => {
    render(<TrackCard track={makeTrack({ AudioPath: "" })} onSelect={vi.fn()} />);
    expect(screen.getByText(/Metadata only/i)).toBeInTheDocument();
  });

  it("downloaded track shows Downloaded badge in bl-red", () => {
    render(
      <TrackCard
        track={makeTrack({ AudioPath: "/tmp/audio.m4a" })}
        onSelect={vi.fn()}
      />,
    );
    const badge = screen.getByText(/Downloaded/i);
    expect(badge.className).toMatch(/bg-bl-red/);
  });
});
