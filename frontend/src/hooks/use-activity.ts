import { useEffect, useRef, useState } from "react";
import { RecentActivity } from "../../wailsjs/go/app/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import type { activity } from "../../wailsjs/go/models";

// -- Constants -----------------------------------------------------------------

const EVENT_NAME = "activity:update";

// -- Public --------------------------------------------------------------------

interface UseActivityResult {
  entries: activity.Entry[];
  loading: boolean;
  error: Error | null;
}

export function useActivity(limit: number): UseActivityResult {
  const [entries, setEntries] = useState<activity.Entry[]>([]);
  const [loading, setLoading] = useState(true);
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
  }, [limit]);

  useEffect(() => {
    const handler = (entry: activity.Entry) => {
      setEntries((prev) => {
        const next = [entry, ...prev.filter((e) => e.id !== entry.id)];
        return next.slice(0, limitRef.current);
      });
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
  }, []);

  return { entries, loading, error };
}
