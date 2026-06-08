import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, beforeEach } from "vitest";
import { App } from "@/App";
import { useUIStore } from "@/stores/ui-store";

describe("App shell", () => {
  beforeEach(() => useUIStore.setState({ view: "library" }));

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

  it("switches view on tab click", () => {
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: /activity/i }));
    expect(screen.getByRole("heading", { name: /activity/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /settings/i }));
    expect(screen.getByRole("heading", { name: /settings/i })).toBeInTheDocument();
  });

  it("active tab highlights with bl-red", () => {
    render(<App />);
    const activityButton = screen.getByRole("button", { name: /activity/i });
    fireEvent.click(activityButton);
    expect(activityButton.className).toMatch(/bl-red/);
  });
});
