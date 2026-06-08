import { useState } from "react";
import { cn } from "@/utils/cn";
import { formatDuration } from "@/utils/format-time";
import type { library } from "../../../wailsjs/go/models";

// -- Interfaces ----------------------------------------------------------------

interface TrackCardProps {
  track: library.Track;
  onSelect: (videoID: string) => void;
}

// -- Constants -----------------------------------------------------------------

const BRIDGE_THUMB_BASE = "http://localhost:7777/thumb";

// -- Components ----------------------------------------------------------------

const TrackCard: React.FC<TrackCardProps> = ({ track, onSelect }) => {
  const [thumbLoaded, setThumbLoaded] = useState(false);
  const isDownloaded = track.AudioPath !== "";
  const aspectClass = track.IsMusic ? "aspect-square" : "aspect-video";

  return (
    <button
      type="button"
      onClick={() => onSelect(track.VideoID)}
      className={cn(
        "group flex flex-col gap-2 rounded-lg border border-border bg-surface p-2 text-left cursor-pointer",
        "transition-all hover:scale-[1.02] hover:border-bl-red-soft",
      )}
      data-testid="track-card"
      data-video-id={track.VideoID}
    >
      <div
        className={cn(
          "relative w-full overflow-hidden rounded-md bg-surface-elevated",
          aspectClass,
        )}
      >
        <img
          src={`${BRIDGE_THUMB_BASE}/${track.VideoID}`}
          alt=""
          loading="lazy"
          onLoad={() => setThumbLoaded(true)}
          className={cn(
            "h-full w-full object-cover transition-opacity",
            thumbLoaded ? "opacity-100" : "opacity-0",
          )}
        />
      </div>
      <div className="flex flex-col gap-0.5">
        <span className="truncate text-sm font-medium text-text" title={track.Title}>
          {track.Title}
        </span>
        <span className="truncate text-xs text-text-muted" title={track.Artist}>
          {track.Artist || "Unknown artist"}
        </span>
        <div className="mt-1 flex items-center justify-between">
          <span className="text-xs text-text-muted">{formatDuration(track.DurationSec)}</span>
          <span
            className={cn(
              "rounded-sm px-1.5 py-0.5 text-[10px] font-medium",
              isDownloaded
                ? "bg-bl-red text-white"
                : "border border-border text-text-muted",
            )}
          >
            {isDownloaded ? "Downloaded" : "Metadata only"}
          </span>
        </div>
      </div>
    </button>
  );
};

export { TrackCard };
