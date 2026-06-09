import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { useBridgeStatus } from "@/hooks/use-bridge-status";
import { useUIStore } from "@/stores/ui-store";
import {
  getWailsRuntime,
  resetWailsMock,
  setupWailsMock,
  type AppBindings,
} from "@/test/wails-mock";
import type { bridgestate } from "../../wailsjs/go/models";

let bindings: AppBindings;

function snapshot(overrides: Partial<bridgestate.State> = {}): bridgestate.State {
  return {
    server: "running",
    download: "idle",
    downloadVideoId: "",
    lastError: "",
    ...overrides,
  } as bridgestate.State;
}

beforeEach(() => {
  bindings = setupWailsMock();
  useUIStore.setState({ bridgeStatus: null });
});

afterEach(() => {
  resetWailsMock();
});

describe("useBridgeStatus", () => {
  it("loads the initial snapshot via BridgeStatus()", async () => {
    bindings.BridgeStatus.mockResolvedValue(snapshot({ server: "running", download: "idle" }));

    const { result } = renderHook(() => useBridgeStatus());

    await waitFor(() => {
      expect(result.current.status?.server).toBe("running");
    });
    expect(result.current.status?.download).toBe("idle");
  });

  it("updates when a bridge:status event fires", async () => {
    bindings.BridgeStatus.mockResolvedValue(snapshot({ server: "running", download: "idle" }));

    const { result } = renderHook(() => useBridgeStatus());
    await waitFor(() => {
      expect(result.current.status).not.toBeNull();
    });

    const runtime = getWailsRuntime();
    type Handler = (s: bridgestate.State) => void;
    const subscription = runtime.EventsOnMultiple.mock.calls.find(
      (call: unknown[]) => call[0] === "bridge:status",
    );
    const handler = subscription?.[1] as Handler | undefined;
    expect(handler).toBeDefined();

    act(() => {
      handler?.(snapshot({ server: "running", download: "active", downloadVideoId: "abc" }));
    });

    expect(result.current.status?.download).toBe("active");
    expect(result.current.status?.downloadVideoId).toBe("abc");
  });

  it("unsubscribes on unmount", async () => {
    bindings.BridgeStatus.mockResolvedValue(snapshot());

    const off: Mock = vi.fn();
    getWailsRuntime().EventsOnMultiple.mockReturnValue(off);

    const { result, unmount } = renderHook(() => useBridgeStatus());
    await waitFor(() => {
      expect(result.current.status).not.toBeNull();
    });

    unmount();
    expect(off).toHaveBeenCalledTimes(1);
  });
});
