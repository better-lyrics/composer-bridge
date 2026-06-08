import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { App } from "@/App";
import { useUIStore } from "@/stores/ui-store";
import { setupWailsMock, resetWailsMock } from "@/test/wails-mock";

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
});
