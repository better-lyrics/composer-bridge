import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { UpdateBanner } from "@/components/update-banner";
import { useUIStore } from "@/stores/ui-store";
import { resetWailsMock, setupWailsMock } from "@/test/wails-mock";
import type { updater } from "../../wailsjs/go/models";

function availableInfo(overrides: Partial<updater.UpdateInfo> = {}): updater.UpdateInfo {
  return {
    available: true,
    current: "1.3.0",
    latest: "1.4.0",
    released_at: "2026-06-10T00:00:00Z",
    notes: "## Bug fixes\n\nFaster downloads.",
    asset: { url: "https://example.invalid/binary", sha256: "deadbeef" },
    ...overrides,
  } as unknown as updater.UpdateInfo;
}

// installMockWithStashedInfo wires the LatestUpdate() Wails call to resolve
// with the same UpdateInfo we're priming the store with. Otherwise the hook's
// onMount effect would resolve null and clobber the store, hiding the banner
// before any assertion ran.
function installMockWithStashedInfo(info: updater.UpdateInfo | null) {
  setupWailsMock({
    LatestUpdate: vi
      .fn<() => Promise<updater.UpdateInfo | null>>()
      .mockResolvedValue(info),
  });
}

beforeEach(() => {
  useUIStore.setState({
    updateInfo: null,
    updateBannerDismissed: false,
  });
  window.localStorage.clear();
});

afterEach(() => {
  cleanup();
  resetWailsMock();
});

describe("UpdateBanner", () => {
  it("renders nothing when no update info is stashed", () => {
    installMockWithStashedInfo(null);
    const { container } = render(<UpdateBanner />);
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when info reports Available=false", async () => {
    const info = availableInfo({ available: false });
    installMockWithStashedInfo(info);
    useUIStore.setState({ updateInfo: info });
    const { container } = render(<UpdateBanner />);
    await waitFor(() => expect(container.firstChild).toBeNull());
  });

  it("shows version label and Install button when an update is available", async () => {
    const info = availableInfo();
    installMockWithStashedInfo(info);
    useUIStore.setState({ updateInfo: info });
    render(<UpdateBanner />);
    await waitFor(() => {
      expect(screen.getByText("Update available")).toBeInTheDocument();
      expect(screen.getByText("v1.4.0")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /^Install$/ })).toBeInTheDocument();
    });
  });

  it("hides on Later click", async () => {
    const info = availableInfo();
    installMockWithStashedInfo(info);
    useUIStore.setState({ updateInfo: info });
    render(<UpdateBanner />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /Later/i })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: /Later/i }));
    await waitFor(() => expect(screen.queryByText("Update available")).toBeNull());
  });

  it("toggles release notes section", async () => {
    const info = availableInfo();
    installMockWithStashedInfo(info);
    useUIStore.setState({ updateInfo: info });
    render(<UpdateBanner />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /What's new/i })).toBeInTheDocument(),
    );
    expect(screen.queryByText(/Faster downloads/)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /What's new/i }));
    expect(screen.getByText(/Faster downloads/)).toBeInTheDocument();
  });

  it("does not render when the user dismissed the session", () => {
    const info = availableInfo();
    installMockWithStashedInfo(info);
    useUIStore.setState({ updateInfo: info, updateBannerDismissed: true });
    const { container } = render(<UpdateBanner />);
    expect(container.firstChild).toBeNull();
  });

  it("hides when the latest version matches the ignored-version in localStorage", () => {
    const info = availableInfo();
    window.localStorage.setItem("composer-bridge:update-banner-ignored-version", info.latest);
    installMockWithStashedInfo(info);
    useUIStore.setState({ updateInfo: info });
    const { container } = render(<UpdateBanner />);
    expect(container.firstChild).toBeNull();
  });
});
