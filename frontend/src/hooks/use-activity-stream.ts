import { useEffect } from "react";
import type { activity } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { useUIStore } from "@/stores/ui-store";

const EVENT_NAME = "activity:update";
const MAX_BUFFERED_ENTRIES = 1000;

// useActivityStream subscribes to activity:update for the lifetime of the
// caller. Mount it once at App level so events update the shared store even
// while the Activity view is unmounted; otherwise a status change in flight
// (running -> ok) only becomes visible after the user reopens the view and
// the next RecentActivity refetch finishes. Caps the buffered list so a long
// session does not grow it without bound.
export function useActivityStream(): void {
  const setEntries = useUIStore((s) => s.setActivityEntries);

  useEffect(() => {
    const handler = (entry: activity.Entry) => {
      const prev = useUIStore.getState().activityEntries ?? [];
      const next = [entry, ...prev.filter((e) => e.id !== entry.id)].slice(
        0,
        MAX_BUFFERED_ENTRIES,
      );
      setEntries(next);
    };
    let off: (() => void) | undefined;
    try {
      off = EventsOn(EVENT_NAME, handler);
    } catch (err) {
      console.error("EventsOn activity:update failed", err);
    }
    return () => {
      if (off) off();
    };
  }, [setEntries]);
}
