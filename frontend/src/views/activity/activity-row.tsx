import {
  IconLoader2,
  IconCheck,
  IconExclamationCircle,
  type IconProps,
} from "@tabler/icons-react";
import { cn } from "@/utils/cn";
import { formatRelativeTime } from "@/utils/format-time";
import type { activity, library } from "../../../wailsjs/go/models";

// -- Interfaces ----------------------------------------------------------------

interface ActivityRowProps {
  entry: activity.Entry;
  trackTitle?: string;
}

// -- Constants -----------------------------------------------------------------

const KIND_LABELS: Record<string, string> = {
  audio_download: "Audio download",
  import: "Import",
  ytdlp_update: "yt-dlp update",
};

// -- Helpers -------------------------------------------------------------------

function statusIcon(status: string): {
  Icon: React.ComponentType<IconProps>;
  className: string;
} {
  if (status === "running") {
    return { Icon: IconLoader2, className: "animate-spin text-bl-red" };
  }
  if (status === "ok") {
    return { Icon: IconCheck, className: "text-bl-red" };
  }
  return { Icon: IconExclamationCircle, className: "text-rose-500" };
}

// -- Components ----------------------------------------------------------------

const ActivityRow: React.FC<ActivityRowProps> = ({ entry, trackTitle }) => {
  const { Icon, className } = statusIcon(entry.status);
  const isLiveDownload =
    entry.kind === "audio_download" && entry.status === "running";
  const kindLabel = KIND_LABELS[entry.kind] ?? entry.kind;
  const subject = trackTitle || entry.video_id || "unknown";
  return (
    <div
      data-testid="activity-row"
      data-status={entry.status}
      data-kind={entry.kind}
      className={cn(
        "flex items-center gap-3 rounded-md border border-border bg-surface px-3 py-2",
        isLiveDownload && "border-l-2 border-l-bl-red",
      )}
    >
      <Icon size={16} className={className} />
      <span className="w-32 shrink-0 text-xs font-medium text-text-muted">{kindLabel}</span>
      <span className="flex-1 truncate text-sm text-text" title={subject}>
        {subject}
      </span>
      {entry.status === "error" && entry.message && (
        <span
          className="truncate text-xs text-rose-400 select-text"
          title={entry.message}
        >
          {entry.message}
        </span>
      )}
      <span className="shrink-0 text-xs text-text-muted">
        {formatRelativeTime(entry.started_at)}
      </span>
    </div>
  );
};

// titleForEntry resolves an entry to a human-readable string from a track lookup
// map. Exported so the activity view can construct the map once and pass per row.
export function titleForEntry(
  entry: activity.Entry,
  tracks: library.Track[],
): string | undefined {
  if (!entry.video_id) return undefined;
  const match = tracks.find((t) => t.video_id === entry.video_id);
  return match?.title;
}

export { ActivityRow };
