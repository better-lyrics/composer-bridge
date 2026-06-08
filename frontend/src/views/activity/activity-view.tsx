import { useMemo, useState } from "react";
import { useActivity } from "@/hooks/use-activity";
import { useLibrary } from "@/hooks/use-library";
import { ActivityRow, titleForEntry } from "@/views/activity/activity-row";
import { Select } from "@/components/select";

// -- Constants -----------------------------------------------------------------

const LIMIT_OPTIONS = [
  { value: "10", label: "Last 10" },
  { value: "50", label: "Last 50" },
  { value: "100", label: "Last 100" },
  { value: "1000", label: "All" },
];

// -- Components ----------------------------------------------------------------

const ActivityView: React.FC = () => {
  const [limit, setLimit] = useState<string>("50");
  const { entries, loading } = useActivity(Number(limit));
  const { tracks } = useLibrary();
  const sorted = useMemo(
    () => entries.toSorted((a, b) => b.started_at - a.started_at),
    [entries],
  );

  return (
    <div className="flex h-full flex-col gap-4">
      <header className="flex items-center justify-between gap-4">
        <h1 className="text-xl font-semibold tracking-tight">Activity</h1>
        <Select
          value={limit}
          onChange={setLimit}
          options={LIMIT_OPTIONS}
          ariaLabel="Activity limit"
        />
      </header>
      {loading && sorted.length === 0 ? (
        <p className="text-sm text-text-muted">Loading...</p>
      ) : sorted.length === 0 ? (
        <p className="text-sm text-text-muted">No activity yet.</p>
      ) : (
        <div className="flex flex-col gap-2">
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
  );
};

export { ActivityView };
