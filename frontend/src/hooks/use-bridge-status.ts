import { useEffect } from "react";
import { BridgeStatus } from "../../wailsjs/go/app/App";
import type { bridgestate } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { useUIStore } from "@/stores/ui-store";

// -- Constants ----------------------------------------------------------------

const EVENT_NAME = "bridge:status";

// -- Public -------------------------------------------------------------------

interface UseBridgeStatusResult {
  status: bridgestate.State | null;
}

export function useBridgeStatus(): UseBridgeStatusResult {
  const status = useUIStore((s) => s.bridgeStatus);
  const setStatus = useUIStore((s) => s.setBridgeStatus);

  useEffect(() => {
    let cancelled = false;
    BridgeStatus()
      .then((next) => {
        if (!cancelled) setStatus(next);
      })
      .catch((err: unknown) => {
        console.error("BridgeStatus failed", err);
      });
    return () => {
      cancelled = true;
    };
  }, [setStatus]);

  useEffect(() => {
    let off: (() => void) | undefined;
    try {
      off = EventsOn(EVENT_NAME, (next: bridgestate.State) => setStatus(next));
    } catch (err) {
      console.error("EventsOn failed", err);
    }
    return () => {
      if (off) off();
    };
  }, [setStatus]);

  return { status };
}
