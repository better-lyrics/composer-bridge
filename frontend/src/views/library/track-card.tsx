import { useState } from "react";
import { IconCheck, IconDownload, IconExternalLink, IconLoader2 } from "@tabler/icons-react";
import { DownloadAudio, OpenInComposer } from "../../../wailsjs/go/app/App";
import type { library } from "../../../wailsjs/go/models";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import { useUIStore } from "@/stores/ui-store";
import { cn } from "@/utils/cn";
import { formatDuration } from "@/utils/format-time";

// -- Interfaces ---------------------------------------------------------------

interface TrackCardProps {
  track: library.Track;
  onSelect: (videoID: string) => void;
  onDownloaded: (track: library.Track) => void;
}

// -- Constants ----------------------------------------------------------------

const BRIDGE_THUMB_BASE = "http://localhost:7777/thumb";

// -- Sub-components -----------------------------------------------------------

const ActionButton: React.FC<{
  label: string;
  onClick: (e: React.MouseEvent) => void;
  disabled?: boolean;
  children: React.ReactNode;
}> = ({ label, onClick, disabled, children }) => (
  <button
    type="button"
    onClick={(e) => {
      e.stopPropagation();
      onClick(e);
    }}
    disabled={disabled}
    aria-label={label}
    title={label}
    className={cn(
      "inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-composer-border bg-composer-button text-composer-text-secondary",
      "transition-colors hover:bg-composer-button-hover hover:text-composer-text",
      "disabled:opacity-50 disabled:hover:bg-composer-button disabled:hover:text-composer-text-secondary",
    )}
  >
    {children}
  </button>
);

// -- Component ----------------------------------------------------------------

const TrackCard: React.FC<TrackCardProps> = ({ track, onSelect, onDownloaded }) => {
  const [thumbLoaded, setThumbLoaded] = useState(false);
  const downloading = useUIStore((s) => s.activeDownloads.has(track.video_id));
  const beginDownload = useUIStore((s) => s.beginDownload);
  const endDownload = useUIStore((s) => s.endDownload);
  const isDownloaded = track.audio_path !== "";

  const handleDownload = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isDownloaded || downloading) return;
    beginDownload(track.video_id);
    try {
      const refreshed = await DownloadAudio(track.video_id);
      onDownloaded(refreshed);
    } catch (err: unknown) {
      console.error("DownloadAudio failed", err);
    } finally {
      endDownload(track.video_id);
    }
  };

  const handleOpenComposer = async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      const url = await OpenInComposer(track.video_id);
      BrowserOpenURL(url);
    } catch (err: unknown) {
      console.error("OpenInComposer failed", err);
    }
  };

  return (
    <button
      type="button"
      onClick={() => onSelect(track.video_id)}
      data-testid="track-card"
      data-video-id={track.video_id}
      className={cn(
        "group flex items-center gap-3 rounded-[0.875rem] border border-composer-border bg-composer-bg-dark p-2 text-left",
        "transition-colors hover:border-composer-border-hover hover:bg-composer-button/30",
      )}
    >
      <div className="relative size-14 shrink-0 overflow-hidden rounded-md bg-composer-bg-elevated">
        <img
          src={`${BRIDGE_THUMB_BASE}/${track.video_id}`}
          alt=""
          loading="lazy"
          onLoad={() => setThumbLoaded(true)}
          className={cn(
            "size-full object-cover transition-opacity",
            thumbLoaded ? "opacity-100" : "opacity-0",
          )}
        />
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-sm font-medium text-composer-text" title={track.title}>
          {track.title}
        </span>
        <span className="truncate text-xs text-composer-text-muted" title={track.artist}>
          {track.artist || "Unknown artist"}
        </span>
        <span className="font-mono text-[11px] text-composer-text-faint">
          {formatDuration(track.duration_sec)}
        </span>
      </div>
      <div className="flex shrink-0 items-center gap-1.5 opacity-0 transition-opacity group-hover:opacity-100">
        <ActionButton
          label={isDownloaded ? "Already downloaded" : downloading ? "Downloading…" : "Download audio"}
          onClick={handleDownload}
          disabled={isDownloaded || downloading}
        >
          {downloading ? (
            <IconLoader2 size={14} className="animate-spin" />
          ) : isDownloaded ? (
            <IconCheck size={14} />
          ) : (
            <IconDownload size={14} />
          )}
        </ActionButton>
        <ActionButton label="Open in Composer" onClick={handleOpenComposer}>
          <IconExternalLink size={14} />
        </ActionButton>
      </div>
    </button>
  );
};

// -- Exports ------------------------------------------------------------------

export { TrackCard };
