import { describe, expect, it, beforeEach, afterEach } from "vitest";
import { fireEvent, render, screen, waitFor, cleanup } from "@testing-library/react";
import { ServerSection } from "@/views/settings/sections/server-section";
import { useUIStore } from "@/stores/ui-store";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";
import type { config } from "../../../../wailsjs/go/models";

function makeConfig(overrides: Partial<config.Config> = {}): config.Config {
  return {
    listen_port: 7777,
    use_random_if_busy: true,
    allowed_origins: [],
    ytdlp_channel: "stable",
    ytdlp_binary_path: "",
    open_at_login: false,
    show_menu_bar_icon: true,
    max_concurrent: 3,
    audio_format: "opus",
    audio_quality: "best",
    log_level: "info",
    data_dir: "",
    download_dir: "",
    server_enabled: true,
    ...overrides,
  } as config.Config;
}

let bindings: AppBindings;

beforeEach(() => {
  bindings = setupWailsMock();
  useUIStore.setState({ bridgeStatus: null });
});

afterEach(() => {
  cleanup();
  resetWailsMock();
});

describe("ServerSection", () => {
  it("toggle is on when status.server is 'running'", () => {
    useUIStore.setState({
      bridgeStatus: {
        server: "running",
        download: "idle",
        downloadVideoId: "",
        lastError: "",
        updatePending: false,
      },
    });
    render(<ServerSection config={makeConfig()} />);
    const toggle = screen.getByRole("switch") as HTMLButtonElement;
    expect(toggle.getAttribute("aria-checked")).toBe("true");
  });

  it("toggle is off when status.server is 'stopped'", () => {
    useUIStore.setState({
      bridgeStatus: {
        server: "stopped",
        download: "idle",
        downloadVideoId: "",
        lastError: "",
        updatePending: false,
      },
    });
    render(<ServerSection config={makeConfig()} />);
    const toggle = screen.getByRole("switch") as HTMLButtonElement;
    expect(toggle.getAttribute("aria-checked")).toBe("false");
  });

  it("calls StartServer when toggled on", async () => {
    useUIStore.setState({
      bridgeStatus: {
        server: "stopped",
        download: "idle",
        downloadVideoId: "",
        lastError: "",
        updatePending: false,
      },
    });
    render(<ServerSection config={makeConfig()} />);
    fireEvent.click(screen.getByRole("switch"));
    await waitFor(() => expect(bindings.StartServer).toHaveBeenCalledTimes(1));
  });

  it("calls StopServer when toggled off", async () => {
    useUIStore.setState({
      bridgeStatus: {
        server: "running",
        download: "idle",
        downloadVideoId: "",
        lastError: "",
        updatePending: false,
      },
    });
    render(<ServerSection config={makeConfig()} />);
    fireEvent.click(screen.getByRole("switch"));
    await waitFor(() => expect(bindings.StopServer).toHaveBeenCalledTimes(1));
  });

  it("disables the toggle during the starting transition", () => {
    useUIStore.setState({
      bridgeStatus: {
        server: "starting",
        download: "idle",
        downloadVideoId: "",
        lastError: "",
        updatePending: false,
      },
    });
    render(<ServerSection config={makeConfig()} />);
    const toggle = screen.getByRole("switch") as HTMLButtonElement;
    expect(toggle).toBeDisabled();
  });

  it("disables the toggle during the stopping transition", () => {
    useUIStore.setState({
      bridgeStatus: {
        server: "stopping",
        download: "idle",
        downloadVideoId: "",
        lastError: "",
        updatePending: false,
      },
    });
    render(<ServerSection config={makeConfig()} />);
    expect(screen.getByRole("switch")).toBeDisabled();
  });

  it("shows the listening URL when running", () => {
    useUIStore.setState({
      bridgeStatus: {
        server: "running",
        download: "idle",
        downloadVideoId: "",
        lastError: "",
        updatePending: false,
      },
    });
    render(<ServerSection config={makeConfig({ listen_port: 7777 })} />);
    expect(screen.getByText(/http:\/\/localhost:7777/)).toBeInTheDocument();
  });
});
