import { useMemo, useState } from "react";
import { Select } from "@/components/select";
import { useActivity } from "@/hooks/use-activity";
import { useLibrary } from "@/hooks/use-library";
import { ActivityRow, titleForEntry } from "@/views/activity/activity-row";

// -- Constants ----------------------------------------------------------------

const LIMIT_OPTIONS = [
  { value: "10", label: "Last 10" },
  { value: "50", label: "Last 50" },
  { value: "100", label: "Last 100" },
  { value: "1000", label: "All" },
];

// -- View ---------------------------------------------------------------------

const ActivityView: React.FC = () => {
  const [limit, setLimit] = useState<string>("50");
  const { entries, loading } = useActivity(Number(limit));
  const { tracks } = useLibrary();
  const sorted = useMemo(() => entries.toSorted((a, b) => b.started_at - a.started_at), [entries]);

  return (
    <div className="flex h-full flex-col">
      <header className="sticky top-0 z-10 flex items-center justify-between gap-4 border-b border-composer-border bg-composer-bg px-6 py-4">
        <div className="flex flex-col gap-0.5">
          <h1 className="text-xl font-semibold tracking-tight text-composer-text">Activity</h1>
          <span className="text-xs text-composer-text-muted">
            {sorted.length} {sorted.length === 1 ? "entry" : "entries"}
          </span>
        </div>
        <Select value={limit} onChange={setLimit} options={LIMIT_OPTIONS} ariaLabel="Activity limit" />
      </header>
      <div className="flex-1 overflow-auto px-6 py-6">
        {loading && sorted.length === 0 ? (
          <p className="text-sm text-composer-text-muted">Loading…</p>
        ) : sorted.length === 0 ? (
          <p className="text-sm text-composer-text-muted">No activity yet.</p>
        ) : (
          <div className="flex flex-col divide-y divide-composer-border rounded-lg border border-composer-border bg-composer-bg-dark overflow-hidden">
            {sorted.map((entry) => (
              <ActivityRow
                key={entry.id}
                entry={entry}
                trackTitle={titleForEntry(entry, tracks)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

// -- Exports ------------------------------------------------------------------

export { ActivityView };
