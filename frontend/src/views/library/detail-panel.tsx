import { useEffect, useState } from "react";
import {
  IconBrandYoutube,
  IconCheck,
  IconDownload,
  IconExternalLink,
  IconLoader2,
  IconTrash,
  IconX,
} from "@tabler/icons-react";
import {
  DownloadAudio,
  GetTrack,
  OpenInComposer,
  OpenInYouTube,
  RemoveTrack,
} from "../../../wailsjs/go/app/App";
import type { library } from "../../../wailsjs/go/models";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import { Button } from "@/components/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { useUIStore } from "@/stores/ui-store";
import { cn } from "@/utils/cn";
import { formatDuration } from "@/utils/format-time";

// -- Constants ----------------------------------------------------------------

const BRIDGE_THUMB_BASE = "http://localhost:7777/thumb";

// -- Interfaces ---------------------------------------------------------------

interface DetailPanelProps {
  onRemoved: () => void;
}

// -- Sub-components -----------------------------------------------------------

const MetaRow: React.FC<{ label: string; value: React.ReactNode; mono?: boolean }> = ({
  label,
  value,
  mono,
}) => (
  <>
    <dt className="text-composer-text-muted">{label}</dt>
    <dd className={cn("text-composer-text select-text", mono && "font-mono text-[11px]")}>{value}</dd>
  </>
);

// -- Component ----------------------------------------------------------------

const TRANSITION_MS = 220;

const DetailPanel: React.FC<DetailPanelProps> = ({ onRemoved }) => {
  const selectedVideoId = useUIStore((s) => s.selectedVideoId);
  const setSelected = useUIStore((s) => s.setSelectedVideoId);
  const [track, setTrack] = useState<library.Track | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const activeId = selectedVideoId ?? track?.video_id ?? "";
  const downloading = useUIStore((s) => activeId !== "" && s.activeDownloads.has(activeId));
  const beginDownload = useUIStore((s) => s.beginDownload);
  const endDownload = useUIStore((s) => s.endDownload);
  const [present, setPresent] = useState(false);
  const [enterPhase, setEnterPhase] = useState(false);
  const isOpen = selectedVideoId !== null;

  useEffect(() => {
    if (!selectedVideoId) return;
    setDownloadError(null);
    setTrack(null);
    let cancelled = false;
    GetTrack(selectedVideoId)
      .then((t) => {
        if (!cancelled) setTrack(t ?? null);
      })
      .catch((err: unknown) => {
        if (!cancelled) console.error("GetTrack failed", err);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedVideoId]);

  useEffect(() => {
    if (isOpen) {
      setPresent(true);
      let cancelled = false;
      requestAnimationFrame(() => {
        if (cancelled) return;
        requestAnimationFrame(() => {
          if (cancelled) return;
          setEnterPhase(true);
        });
      });
      return () => {
        cancelled = true;
      };
    }
    setEnterPhase(false);
    const t = setTimeout(() => setPresent(false), TRANSITION_MS);
    return () => clearTimeout(t);
  }, [isOpen]);

  if (!present) return null;

  const handleClose = () => setSelected(null);

  const openComposer = async () => {
    if (!track) return;
    const url = await OpenInComposer(track.video_id);
    BrowserOpenURL(url);
  };

  const openYouTube = async () => {
    if (!track) return;
    const url = await OpenInYouTube(track.video_id);
    BrowserOpenURL(url);
  };

  const downloadAudio = async () => {
    if (!track) return;
    beginDownload(track.video_id);
    setDownloadError(null);
    try {
      const refreshed = await DownloadAudio(track.video_id);
      setTrack(refreshed);
    } catch (err: unknown) {
      setDownloadError(err instanceof Error ? err.message : String(err));
    } finally {
      endDownload(track.video_id);
    }
  };

  const handleRemoveConfirmed = async () => {
    if (!track) return;
    try {
      await RemoveTrack(track.video_id);
    } catch (err: unknown) {
      console.error("RemoveTrack failed", err);
    }
    setConfirmOpen(false);
    setSelected(null);
    onRemoved();
  };

  const aspectClass = track?.is_music ? "aspect-square" : "aspect-video";
  const isDownloaded = Boolean(track?.audio_path && track.audio_path.length > 0);

  return (
    <>
      <div
        onClick={handleClose}
        aria-hidden="true"
        className={cn(
          "fixed inset-0 z-30 bg-black/60 backdrop-blur-sm transition-opacity ease-out",
          enterPhase ? "opacity-100" : "opacity-0",
        )}
        style={{ transitionDuration: `${TRANSITION_MS}ms` }}
      />
      <aside
        role="complementary"
        aria-label="Track details"
        className={cn(
          "fixed top-0 right-0 z-40 flex h-full w-[400px] flex-col border-l border-composer-border bg-composer-bg-dark shadow-2xl",
          "transition-transform ease-out",
          enterPhase ? "translate-x-0" : "translate-x-full",
        )}
        style={{ transitionDuration: `${TRANSITION_MS}ms` }}
      >
        <header className="flex items-center justify-between border-b border-composer-border px-5 py-3">
          <span className="text-xs tracking-wide text-composer-text-muted">Track details</span>
          <Button variant="ghost" size="icon" aria-label="Close details" onClick={handleClose}>
            <IconX size={16} />
          </Button>
        </header>
        {!track && (
          <div className="flex flex-1 items-center justify-center text-sm text-composer-text-muted">
            Loading…
          </div>
        )}
        {track && (
          <div className="flex flex-col gap-5 overflow-y-auto px-5 py-5">
            <div className={cn("w-full overflow-hidden rounded-lg bg-composer-bg-elevated", aspectClass)}>
              <img
                src={`${BRIDGE_THUMB_BASE}/${track.video_id}`}
                alt=""
                className="h-full w-full object-cover"
              />
            </div>
            <div className="flex flex-col gap-0.5">
              <h2 className="text-base font-semibold text-composer-text select-text">{track.title}</h2>
              <p className="text-sm text-composer-text-secondary select-text">
                {track.artist || "Unknown artist"}
              </p>
            </div>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 text-xs">
              {track.album && <MetaRow label="Album" value={track.album} />}
              <MetaRow label="Duration" value={formatDuration(track.duration_sec)} mono />
              {track.release_year > 0 && <MetaRow label="Released" value={track.release_year} />}
              {track.music_type && <MetaRow label="Type" value={track.music_type} />}
              <MetaRow label="Video ID" value={track.video_id} mono />
            </dl>
            <div className="flex flex-col gap-2">
              <Button variant="primary" size="md" hasIcon onClick={openComposer}>
                <IconExternalLink size={14} />
                Open in Composer
              </Button>
              <Button
                variant="secondary"
                size="md"
                hasIcon
                onClick={downloadAudio}
                disabled={downloading || isDownloaded}
              >
                {downloading ? (
                  <IconLoader2 size={14} className="animate-spin" />
                ) : isDownloaded ? (
                  <IconCheck size={14} />
                ) : (
                  <IconDownload size={14} />
                )}
                {downloading ? "Downloading…" : isDownloaded ? "Audio downloaded" : "Download audio"}
              </Button>
              {downloadError && (
                <span className="text-xs text-composer-error-text">{downloadError}</span>
              )}
              <Button variant="secondary" size="md" hasIcon onClick={openYouTube}>
                <IconBrandYoutube size={14} />
                Open in YouTube
              </Button>
              <Button
                variant="ghost"
                size="md"
                hasIcon
                onClick={() => setConfirmOpen(true)}
                className="text-composer-text-muted hover:text-composer-error-text"
              >
                <IconTrash size={14} />
                Remove from library
              </Button>
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

// -- Exports ------------------------------------------------------------------

export { DetailPanel };
