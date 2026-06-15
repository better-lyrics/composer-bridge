import { render, screen, fireEvent, cleanup, waitFor, act } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { App } from "@/App";
import { useUIStore } from "@/stores/ui-store";
import { getWailsRuntime, setupWailsMock, resetWailsMock } from "@/test/wails-mock";

describe("App shell", () => {
  beforeEach(() => {
    useUIStore.setState({
      view: "library",
      selectedVideoId: null,
      librarySearch: "",
      librarySort: "recent",
    });
    setupWailsMock();
  });

  afterEach(() => {
    cleanup();
    resetWailsMock();
  });

  it("renders all three sidebar tabs", () => {
    render(<App />);
    expect(screen.getByRole("button", { name: /library/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /activity/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /settings/i })).toBeInTheDocument();
  });

  it("starts on the library view", () => {
    render(<App />);
    expect(screen.getByRole("heading", { name: /library/i })).toBeInTheDocument();
  });

  it("switches view on tab click", async () => {
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: /activity/i }));
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: /activity/i })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: /settings/i }));
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: /settings/i })).toBeInTheDocument(),
    );
  });

  it("active tab uses the composer button highlight", () => {
    render(<App />);
    const activityButton = screen.getByRole("button", { name: /activity/i });
    fireEvent.click(activityButton);
    expect(activityButton.className).toMatch(/bg-composer-button/);
    expect(activityButton.getAttribute("aria-current")).toBe("page");
  });

  it("subscribes to activity:update at App mount so events flow while Activity is unmounted", () => {
    useUIStore.setState({ activityEntries: null });
    render(<App />);
    const runtime = getWailsRuntime();
    const names = runtime.EventsOnMultiple.mock.calls.map(([name]) => name);
    expect(names).toContain("activity:update");
  });

  it("activity:update updates the store even when the Activity view is not mounted", () => {
    useUIStore.setState({ activityEntries: null });
    render(<App />);
    const runtime = getWailsRuntime();
    const subscription = runtime.EventsOnMultiple.mock.calls.find(
      ([name]) => name === "activity:update",
    );
    if (!subscription) throw new Error("App did not subscribe to activity:update");
    const handler = subscription[1] as (entry: unknown) => void;
    const entry = {
      id: 42,
      kind: "audio_download",
      video_id: "RgKAFK5djSk",
      started_at: Date.now(),
      ended_at: 0,
      status: "running",
      message: "",
    };
    act(() => {
      handler(entry);
    });
    const stored = useUIStore.getState().activityEntries ?? [];
    expect(stored.some((e) => e.id === 42 && e.status === "running")).toBe(true);
  });
});
