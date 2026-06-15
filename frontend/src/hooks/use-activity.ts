import { useEffect, useMemo, useState } from "react";
import { RecentActivity } from "../../wailsjs/go/app/App";
import type { activity } from "../../wailsjs/go/models";
import { useUIStore } from "@/stores/ui-store";

// -- Public -------------------------------------------------------------------

interface UseActivityResult {
  entries: activity.Entry[];
  loaded: boolean;
  loading: boolean;
  error: Error | null;
}

// useActivity exposes the most recent `limit` activity rows to the Activity
// view. Reads from the shared store so live events captured by
// useActivityStream (mounted at the App root) are visible immediately; the
// initial RecentActivity fetch backfills history on first mount and on limit
// changes so the SQLite snapshot wins when it differs from the in-memory
// buffer.
export function useActivity(limit: number): UseActivityResult {
  const entries = useUIStore((s) => s.activityEntries);
  const setEntries = useUIStore((s) => s.setActivityEntries);
  const [loading, setLoading] = useState(entries === null);
  const [error, setError] = useState<Error | null>(null);

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

  const limited = useMemo(() => (entries ?? []).slice(0, limit), [entries, limit]);

  return {
    entries: entries === null ? [] : limited,
    loaded: entries !== null,
    loading,
    error,
  };
}
