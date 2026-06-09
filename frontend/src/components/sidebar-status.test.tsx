import { render, screen, cleanup } from "@testing-library/react";
import { describe, expect, it, beforeEach, afterEach } from "vitest";
import { SidebarStatus } from "@/components/sidebar-status";
import { useUIStore } from "@/stores/ui-store";
import type { bridgestate } from "../../wailsjs/go/models";

function state(overrides: Partial<bridgestate.State> = {}): bridgestate.State {
  return {
    server: "running",
    download: "idle",
    downloadVideoId: "",
    lastError: "",
    ...overrides,
  } as bridgestate.State;
}

beforeEach(() => {
  useUIStore.setState({ bridgeStatus: null, view: "library" });
});

afterEach(() => {
  cleanup();
});

describe("SidebarStatus", () => {
  it("renders nothing while the status is still loading", () => {
    const { container } = render(<SidebarStatus />);
    expect(container.firstChild).toBeNull();
  });

  it("shows 'Online' when server is running and download is idle", () => {
    useUIStore.setState({ bridgeStatus: state({ server: "running", download: "idle" }) });
    render(<SidebarStatus />);
    expect(screen.getByText("Online")).toBeInTheDocument();
  });

  it("shows 'Downloading {videoId}' when download is active", () => {
    useUIStore.setState({
      bridgeStatus: state({ server: "running", download: "active", downloadVideoId: "abc123" }),
    });
    render(<SidebarStatus />);
    expect(screen.getByText(/Downloading/)).toBeInTheDocument();
    expect(screen.getByText(/abc123/)).toBeInTheDocument();
  });

  it("shows 'Server stopped' when server is stopped", () => {
    useUIStore.setState({ bridgeStatus: state({ server: "stopped", download: "idle" }) });
    render(<SidebarStatus />);
    expect(screen.getByText("Server stopped")).toBeInTheDocument();
  });

  it("shows 'Starting...' during server starting", () => {
    useUIStore.setState({ bridgeStatus: state({ server: "starting", download: "idle" }) });
    render(<SidebarStatus />);
    expect(screen.getByText("Starting...")).toBeInTheDocument();
  });

  it("shows 'Stopping...' during server stopping", () => {
    useUIStore.setState({ bridgeStatus: state({ server: "stopping", download: "idle" }) });
    render(<SidebarStatus />);
    expect(screen.getByText("Stopping...")).toBeInTheDocument();
  });

  it("navigates to Settings when clicked", () => {
    useUIStore.setState({ bridgeStatus: state({ server: "running", download: "idle" }) });
    render(<SidebarStatus />);
    screen.getByRole("button").click();
    expect(useUIStore.getState().view).toBe("settings");
  });
});
