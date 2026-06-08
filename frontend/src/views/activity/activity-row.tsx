import {
  IconCheck,
  IconExclamationCircle,
  IconLoader2,
  type IconProps,
} from "@tabler/icons-react";
import type { activity, library } from "../../../wailsjs/go/models";
import { cn } from "@/utils/cn";
import { formatRelativeTime } from "@/utils/format-time";

// -- Interfaces ---------------------------------------------------------------

interface ActivityRowProps {
  entry: activity.Entry;
  trackTitle?: string;
}

// -- Constants ----------------------------------------------------------------

const KIND_LABELS: Record<string, string> = {
  audio_download: "Audio download",
  import: "Import",
  ytdlp_update: "yt-dlp update",
};

// -- Helpers ------------------------------------------------------------------

function statusIcon(status: string): {
  Icon: React.ComponentType<IconProps>;
  className: string;
} {
  if (status === "running") {
    return { Icon: IconLoader2, className: "animate-spin text-composer-accent" };
  }
  if (status === "ok") {
    return { Icon: IconCheck, className: "text-composer-success" };
  }
  return { Icon: IconExclamationCircle, className: "text-composer-error-text" };
}

// -- Component ----------------------------------------------------------------

const ActivityRow: React.FC<ActivityRowProps> = ({ entry, trackTitle }) => {
  const { Icon, className } = statusIcon(entry.status);
  const isLiveDownload = entry.kind === "audio_download" && entry.status === "running";
  const kindLabel = KIND_LABELS[entry.kind] ?? entry.kind;
  const subject = trackTitle || entry.video_id || "unknown";
  return (
    <div
      data-testid="activity-row"
      data-status={entry.status}
      data-kind={entry.kind}
      className={cn(
        "flex items-center gap-3 px-4 py-3 transition-colors hover:bg-composer-button/40",
        isLiveDownload && "bg-composer-accent/5",
      )}
    >
      <Icon size={16} className={className} />
      <span className="w-32 shrink-0 text-xs font-medium uppercase tracking-wider text-composer-text-muted">
        {kindLabel}
      </span>
      <span className="flex-1 truncate text-sm text-composer-text" title={subject}>
        {subject}
      </span>
      {entry.status === "error" && entry.message && (
        <span
          className="max-w-xs truncate text-xs text-composer-error-text/80 select-text"
          title={entry.message}
        >
          {entry.message}
        </span>
      )}
      <span className="shrink-0 font-mono text-[11px] text-composer-text-muted">
        {formatRelativeTime(entry.started_at)}
      </span>
    </div>
  );
};

// titleForEntry resolves an entry to a human-readable string from a track lookup
// map. Exported so the activity view can construct the map once and pass per row.
export function titleForEntry(entry: activity.Entry, tracks: library.Track[]): string | undefined {
  if (!entry.video_id) return undefined;
  const match = tracks.find((t) => t.video_id === entry.video_id);
  return match?.title;
}

// -- Exports ------------------------------------------------------------------

export { ActivityRow };
