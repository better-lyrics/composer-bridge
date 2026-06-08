import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { SettingsView } from "@/views/settings/settings-view";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";

let bindings: AppBindings;

beforeEach(() => {
  bindings = setupWailsMock();
});

afterEach(() => {
  cleanup();
  resetWailsMock();
});

describe("SettingsView", () => {
  it("renders all five sections", async () => {
    render(<SettingsView />);
    await waitFor(() => expect(screen.getByText("Networking")).toBeInTheDocument());
    expect(screen.getByText("yt-dlp")).toBeInTheDocument();
    expect(screen.getByText("Storage")).toBeInTheDocument();
    expect(screen.getByText("Behavior")).toBeInTheDocument();
    expect(screen.getByText("Diagnostics")).toBeInTheDocument();
  });

  it("editing the listen port triggers a debounced SaveConfig call", async () => {
    render(<SettingsView />);
    const portInput = (await screen.findByLabelText(/Listen port/i)) as HTMLInputElement;
    fireEvent.change(portInput, { target: { value: "8081" } });
    await waitFor(
      () => expect(bindings.SaveConfig).toHaveBeenCalledTimes(1),
      { timeout: 1500 },
    );
    const arg = bindings.SaveConfig.mock.calls[0]?.[0];
    expect(arg).toMatchObject({ listen_port: 8081 });
  });

  it("flipping use_random_if_busy triggers a debounced SaveConfig call", async () => {
    render(<SettingsView />);
    const toggle = await screen.findByLabelText(/Use random port if busy/i);
    fireEvent.click(toggle);
    await waitFor(
      () => expect(bindings.SaveConfig).toHaveBeenCalledTimes(1),
      { timeout: 1500 },
    );
    const arg = bindings.SaveConfig.mock.calls[0]?.[0];
    expect(arg).toMatchObject({ use_random_if_busy: false });
  });
});
