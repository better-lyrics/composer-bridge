import { useState } from "react";
import {
  IconCheck,
  IconCopy,
  IconExclamationCircle,
  IconLoader2,
  type IconProps,
} from "@tabler/icons-react";
import type { activity } from "../../../wailsjs/go/models";
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

// -- Sub-components -----------------------------------------------------------

const CopyButton: React.FC<{ text: string }> = ({ text }) => {
  const [copied, setCopied] = useState(false);
  const handle = async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch (err) {
      console.error("clipboard write failed", err);
    }
  };
  return (
    <button
      type="button"
      onClick={handle}
      aria-label="Copy error"
      title="Copy error"
      className="shrink-0 rounded p-1 text-composer-text-muted hover:bg-composer-button hover:text-composer-text transition-colors"
    >
      {copied ? <IconCheck size={12} /> : <IconCopy size={12} />}
    </button>
  );
};

// -- Component ----------------------------------------------------------------

const ActivityRow: React.FC<ActivityRowProps> = ({ entry, trackTitle }) => {
  const { Icon, className } = statusIcon(entry.status);
  const isLiveDownload = entry.kind === "audio_download" && entry.status === "running";
  const kindLabel = KIND_LABELS[entry.kind] ?? entry.kind;
  const subject = trackTitle || entry.video_id || "unknown";
  const hasError = entry.status === "error" && Boolean(entry.message);
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
      <span className="w-32 shrink-0 text-xs font-medium tracking-wide text-composer-text-muted">
        {kindLabel}
      </span>
      <span className="flex-1 truncate text-sm text-composer-text select-text" title={subject}>
        {subject}
      </span>
      {hasError && (
        <>
          <span
            className="max-w-xs truncate text-xs text-composer-error-text/80 select-text"
            title={entry.message}
          >
            {entry.message}
          </span>
          <CopyButton text={entry.message} />
        </>
      )}
      <span className="shrink-0 font-mono text-[11px] text-composer-text-muted">
        {formatRelativeTime(entry.started_at)}
      </span>
    </div>
  );
};

// titleForEntry resolves an entry to a human-readable string from a track lookup
// map. The view builds the map once with useMemo so this is O(1) per row.
export function titleForEntry(
  entry: activity.Entry,
  titleByVideoId: Map<string, string>,
): string | undefined {
  if (!entry.video_id) return undefined;
  return titleByVideoId.get(entry.video_id);
}

// -- Exports ------------------------------------------------------------------

export { ActivityRow };
