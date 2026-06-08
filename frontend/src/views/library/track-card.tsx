import { useState } from "react";
import type { library } from "../../../wailsjs/go/models";
import { cn } from "@/utils/cn";
import { formatDuration } from "@/utils/format-time";

// -- Interfaces ---------------------------------------------------------------

interface TrackCardProps {
  track: library.Track;
  onSelect: (videoID: string) => void;
}

// -- Constants ----------------------------------------------------------------

const BRIDGE_THUMB_BASE = "http://localhost:7777/thumb";

// -- Component ----------------------------------------------------------------

const TrackCard: React.FC<TrackCardProps> = ({ track, onSelect }) => {
  const [thumbLoaded, setThumbLoaded] = useState(false);
  const isDownloaded = track.audio_path !== "";
  const aspectClass = track.is_music ? "aspect-square" : "aspect-video";

  return (
    <button
      type="button"
      onClick={() => onSelect(track.video_id)}
      data-testid="track-card"
      data-video-id={track.video_id}
      className={cn(
        "group flex flex-col gap-2 rounded-lg border border-composer-border bg-composer-bg-dark p-2 text-left cursor-pointer",
        "transition-colors hover:border-composer-border-hover",
      )}
    >
      <div className={cn("relative w-full overflow-hidden rounded-md bg-composer-bg-elevated", aspectClass)}>
        <img
          src={`${BRIDGE_THUMB_BASE}/${track.video_id}`}
          alt=""
          loading="lazy"
          onLoad={() => setThumbLoaded(true)}
          className={cn(
            "h-full w-full object-cover transition-opacity",
            thumbLoaded ? "opacity-100" : "opacity-0",
          )}
        />
      </div>
      <div className="flex flex-col gap-0.5 px-1">
        <span className="truncate text-sm font-medium text-composer-text" title={track.title}>
          {track.title}
        </span>
        <span className="truncate text-xs text-composer-text-muted" title={track.artist}>
          {track.artist || "Unknown artist"}
        </span>
        <div className="mt-1 flex items-center justify-between">
          <span className="font-mono text-[11px] text-composer-text-muted">
            {formatDuration(track.duration_sec)}
          </span>
          <span
            className={cn(
              "rounded px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider",
              isDownloaded
                ? "bg-composer-accent/15 text-composer-accent-text"
                : "text-composer-text-faint",
            )}
          >
            {isDownloaded ? "Downloaded" : "Metadata"}
          </span>
        </div>
      </div>
    </button>
  );
};

// -- Exports ------------------------------------------------------------------

export { TrackCard };
