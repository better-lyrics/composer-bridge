import { useEffect, useRef, useState } from "react";
import { RecentActivity } from "../../wailsjs/go/app/App";
import type { activity } from "../../wailsjs/go/models";
import { EventsOff, EventsOn } from "../../wailsjs/runtime/runtime";
import { useUIStore } from "@/stores/ui-store";

// -- Constants ----------------------------------------------------------------

const EVENT_NAME = "activity:update";

// -- Public -------------------------------------------------------------------

interface UseActivityResult {
  entries: activity.Entry[];
  loaded: boolean;
  loading: boolean;
  error: Error | null;
}

export function useActivity(limit: number): UseActivityResult {
  const entries = useUIStore((s) => s.activityEntries);
  const setEntries = useUIStore((s) => s.setActivityEntries);
  const [loading, setLoading] = useState(entries === null);
  const [error, setError] = useState<Error | null>(null);
  const limitRef = useRef(limit);
  limitRef.current = limit;

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    RecentActivity(limit)
      .then((rows) => {
        if (cancelled) return;
        setEntries(rows ?? []);
        setError(null);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err : new Error(String(err)));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [limit, setEntries]);

  useEffect(() => {
    const handler = (entry: activity.Entry) => {
      const prev = useUIStore.getState().activityEntries ?? [];
      const next = [entry, ...prev.filter((e) => e.id !== entry.id)].slice(0, limitRef.current);
      setEntries(next);
    };
    try {
      EventsOn(EVENT_NAME, handler);
    } catch (err) {
      console.error("EventsOn failed", err);
    }
    return () => {
      try {
        EventsOff(EVENT_NAME);
      } catch (err) {
        console.error("EventsOff failed", err);
      }
    };
  }, [setEntries]);

  return {
    entries: entries ?? [],
    loaded: entries !== null,
    loading,
    error,
  };
}
