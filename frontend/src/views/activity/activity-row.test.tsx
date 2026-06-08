import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ActivityRow } from "@/views/activity/activity-row";
import type { activity } from "../../../wailsjs/go/models";

function entry(overrides: Partial<activity.Entry> = {}): activity.Entry {
  return {
    id: 1,
    kind: "audio_download",
    video_id: "RgKAFK5djSk",
    started_at: Date.now() - 5000,
    ended_at: 0,
    status: "running",
    message: "",
    ...overrides,
  } as activity.Entry;
}

describe("ActivityRow", () => {
  it("formats started_at as a relative time", () => {
    const now = Date.now();
    render(<ActivityRow entry={entry({ started_at: now - 30_000 })} />);
    expect(screen.getByText(/^\d+s ago$/)).toBeInTheDocument();
  });

  it("running audio_download row carries the composer-accent tint", () => {
    render(<ActivityRow entry={entry({ status: "running", kind: "audio_download" })} />);
    const row = screen.getByTestId("activity-row");
    expect(row.className).toMatch(/bg-composer-accent\/5/);
  });

  it("non-running audio_download row does NOT carry the live tint", () => {
    render(<ActivityRow entry={entry({ status: "ok", kind: "audio_download" })} />);
    const row = screen.getByTestId("activity-row");
    expect(row.className).not.toMatch(/bg-composer-accent\/5/);
  });
});
