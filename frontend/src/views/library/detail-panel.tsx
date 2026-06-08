import { useEffect, useState } from "react";
import { IconBrandYoutube, IconExternalLink, IconTrash, IconDownload, IconX } from "@tabler/icons-react";
import { GetTrack, OpenInComposer, OpenInYouTube, RemoveTrack } from "../../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import type { library } from "../../../wailsjs/go/models";
import { useUIStore } from "@/stores/ui-store";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { cn } from "@/utils/cn";
import { formatDuration } from "@/utils/format-time";

// -- Constants -----------------------------------------------------------------

const BRIDGE_THUMB_BASE = "http://localhost:7777/thumb";

// -- Interfaces ----------------------------------------------------------------

interface DetailPanelProps {
  onRemoved: () => void;
}

// -- Components ----------------------------------------------------------------

const DetailPanel: React.FC<DetailPanelProps> = ({ onRemoved }) => {
  const selectedVideoId = useUIStore((s) => s.selectedVideoId);
  const setSelected = useUIStore((s) => s.setSelectedVideoId);
  const [track, setTrack] = useState<library.Track | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const isOpen = selectedVideoId !== null;

  useEffect(() => {
    if (!selectedVideoId) {
      setTrack(null);
      return;
    }
    let cancelled = false;
    GetTrack(selectedVideoId)
      .then((t) => {
        if (!cancelled) setTrack(t ?? null);
      })
      .catch((err) => {
        if (!cancelled) console.error("GetTrack failed", err);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedVideoId]);

  if (!isOpen) return null;

  const handleClose = () => setSelected(null);

  const openComposer = async () => {
    if (!track) return;
    try {
      const url = await OpenInComposer(track.VideoID);
      BrowserOpenURL(url);
    } catch (err) {
      console.error("OpenInComposer failed", err);
    }
  };

  const openYouTube = async () => {
    if (!track) return;
    try {
      const url = await OpenInYouTube(track.VideoID);
      BrowserOpenURL(url);
    } catch (err) {
      console.error("OpenInYouTube failed", err);
    }
  };

  const handleRemoveConfirmed = async () => {
    if (!track) return;
    try {
      await RemoveTrack(track.VideoID);
    } catch (err) {
      console.error("RemoveTrack failed", err);
    }
    setConfirmOpen(false);
    setSelected(null);
    onRemoved();
  };

  const aspectClass = track?.IsMusic ? "aspect-square" : "aspect-video";

  return (
    <>
      <div
        className="fixed inset-0 z-30 bg-black/40"
        onClick={handleClose}
        aria-hidden="true"
      />
      <aside
        role="complementary"
        aria-label="Track details"
        className={cn(
          "fixed top-0 right-0 z-40 flex h-full w-96 flex-col border-l border-border bg-surface p-5",
          "transition-transform translate-x-0",
        )}
      >
        <div className="mb-4 flex items-center justify-between">
          <span className="text-sm font-medium text-text-muted">Track details</span>
          <button
            type="button"
            onClick={handleClose}
            aria-label="Close details"
            className="rounded-md p-1 text-text-muted cursor-pointer hover:bg-bl-red-soft hover:text-text"
          >
            <IconX size={16} />
          </button>
        </div>
        {!track && <div className="text-sm text-text-muted">Loading...</div>}
        {track && (
          <div className="flex flex-col gap-4 overflow-y-auto">
            <div className={cn("w-full overflow-hidden rounded-md bg-surface-elevated", aspectClass)}>
              <img
                src={`${BRIDGE_THUMB_BASE}/${track.VideoID}`}
                alt=""
                className="h-full w-full object-cover"
              />
            </div>
            <div className="flex flex-col gap-1">
              <h2 className="text-base font-semibold text-text select-text">{track.Title}</h2>
              <p className="text-sm text-text-muted select-text">{track.Artist || "Unknown artist"}</p>
            </div>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 text-xs">
              {track.Album && (
                <>
                  <dt className="text-text-muted">Album</dt>
                  <dd className="text-text select-text">{track.Album}</dd>
                </>
              )}
              <dt className="text-text-muted">Duration</dt>
              <dd className="text-text select-text">{formatDuration(track.DurationSec)}</dd>
              {track.ReleaseYear > 0 && (
                <>
                  <dt className="text-text-muted">Released</dt>
                  <dd className="text-text select-text">{track.ReleaseYear}</dd>
                </>
              )}
              {track.MusicType && (
                <>
                  <dt className="text-text-muted">Type</dt>
                  <dd className="text-text select-text">{track.MusicType}</dd>
                </>
              )}
              <dt className="text-text-muted">Source</dt>
              <dd className="truncate text-text select-text" title={track.SourceURL}>
                {track.SourceURL}
              </dd>
            </dl>
            <div className="mt-2 flex flex-col gap-2">
              <button
                type="button"
                disabled
                title="Audio download wiring lands in a follow-up."
                className={cn(
                  "flex items-center gap-2 rounded-md border border-border bg-surface-elevated px-3 py-2 text-sm",
                  "text-text-muted cursor-not-allowed opacity-60",
                )}
              >
                <IconDownload size={14} />
                {track.AudioPath === "" ? "Download audio" : "Audio downloaded"}
              </button>
              <button
                type="button"
                onClick={openComposer}
                className={cn(
                  "flex items-center gap-2 rounded-md bg-bl-red px-3 py-2 text-sm font-medium text-white cursor-pointer",
                  "hover:bg-bl-red-hover",
                )}
              >
                <IconExternalLink size={14} />
                Open in Composer
              </button>
              <button
                type="button"
                onClick={openYouTube}
                className={cn(
                  "flex items-center gap-2 rounded-md border border-border bg-surface-elevated px-3 py-2 text-sm text-text cursor-pointer",
                  "hover:border-bl-red-soft",
                )}
              >
                <IconBrandYoutube size={14} />
                Open in YouTube
              </button>
              <button
                type="button"
                onClick={() => setConfirmOpen(true)}
                className={cn(
                  "flex items-center gap-2 rounded-md border border-border bg-surface-elevated px-3 py-2 text-sm text-text-muted cursor-pointer",
                  "hover:border-bl-red hover:text-bl-red",
                )}
              >
                <IconTrash size={14} />
                Remove from library
              </button>
            </div>
          </div>
        )}
      </aside>
      <ConfirmDialog
        open={confirmOpen}
        title="Remove this track?"
        description="The library entry will be deleted. Audio files on disk are unaffected."
        confirmLabel="Remove"
        cancelLabel="Cancel"
        destructive
        onConfirm={handleRemoveConfirmed}
        onCancel={() => setConfirmOpen(false)}
      />
    </>
  );
};

export { DetailPanel };
