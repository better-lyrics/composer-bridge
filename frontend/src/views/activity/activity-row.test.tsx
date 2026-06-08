import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ActivityRow } from "@/views/activity/activity-row";
import type { activity } from "../../../wailsjs/go/models";

function entry(overrides: Partial<activity.Entry> = {}): activity.Entry {
  return {
    ID: 1,
    Kind: "audio_download",
    VideoID: "RgKAFK5djSk",
    StartedAt: Date.now() - 5000,
    EndedAt: 0,
    Status: "running",
    Message: "",
    ...overrides,
  } as activity.Entry;
}

describe("ActivityRow", () => {
  it("formats started_at as a relative time", () => {
    const now = Date.now();
    render(<ActivityRow entry={entry({ StartedAt: now - 30_000 })} />);
    expect(screen.getByText(/^\d+s ago$/)).toBeInTheDocument();
  });

  it("running audio_download row gets a bl-red left border", () => {
    render(<ActivityRow entry={entry({ Status: "running", Kind: "audio_download" })} />);
    const row = screen.getByTestId("activity-row");
    expect(row.className).toMatch(/border-l-bl-red/);
  });

  it("non-running audio_download row does NOT get the live border", () => {
    render(<ActivityRow entry={entry({ Status: "ok", Kind: "audio_download" })} />);
    const row = screen.getByTestId("activity-row");
    expect(row.className).not.toMatch(/border-l-bl-red/);
  });
});
