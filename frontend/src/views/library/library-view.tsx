import { useState } from "react";
import { IconSearch, IconCopy, IconCheck } from "@tabler/icons-react";
import { useLibrary } from "@/hooks/use-library";
import { useUIStore, type LibrarySort } from "@/stores/ui-store";
import { TrackCard } from "@/views/library/track-card";
import { DetailPanel } from "@/views/library/detail-panel";
import { Select } from "@/components/select";
import { TextInput } from "@/components/text-input";
import { cn } from "@/utils/cn";

// -- Constants -----------------------------------------------------------------

const IMPORT_URL = "http://localhost:7777/import";

const SORT_OPTIONS: { value: LibrarySort; label: string }[] = [
  { value: "recent", label: "Recently imported" },
  { value: "title", label: "Title A-Z" },
  { value: "artist", label: "Artist A-Z" },
  { value: "duration", label: "Duration" },
];

// -- Components ----------------------------------------------------------------

const EmptyState: React.FC = () => {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(IMPORT_URL);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch (err) {
      console.error("clipboard write failed", err);
    }
  };
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 py-20 text-center">
      <p className="text-sm text-text-muted">No tracks yet. Use POST /import to add some.</p>
      <button
        type="button"
        onClick={copy}
        className={cn(
          "flex items-center gap-2 rounded-md border border-bl-red bg-transparent px-3 py-1.5 text-sm font-medium text-bl-red cursor-pointer",
          "hover:bg-bl-red-soft",
        )}
      >
        {copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
        {copied ? "Copied" : `Copy ${IMPORT_URL}`}
      </button>
    </div>
  );
};

const LibraryView: React.FC = () => {
  const { tracks, reload } = useLibrary();
  const sort = useUIStore((s) => s.librarySort);
  const setSort = useUIStore((s) => s.setLibrarySort);
  const search = useUIStore((s) => s.librarySearch);
  const setSearch = useUIStore((s) => s.setLibrarySearch);
  const setSelected = useUIStore((s) => s.setSelectedVideoId);

  return (
    <div className="flex h-full flex-col gap-4">
      <header className="flex items-center justify-between gap-4">
        <h1 className="text-xl font-semibold tracking-tight">Library</h1>
        <div className="flex items-center gap-3">
          <div className="relative">
            <IconSearch
              size={14}
              className="absolute top-1/2 left-2.5 -translate-y-1/2 text-text-muted"
            />
            <TextInput
              value={search}
              onChange={setSearch}
              placeholder="Search title, artist, album"
              ariaLabel="Search library"
              className="w-64 pl-8"
            />
          </div>
          <Select
            value={sort}
            onChange={setSort}
            options={SORT_OPTIONS}
            ariaLabel="Sort tracks"
          />
        </div>
      </header>
      {tracks.length === 0 ? (
        <EmptyState />
      ) : (
        <div
          className="grid gap-4"
          style={{ gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))" }}
        >
          {tracks.map((track) => (
            <TrackCard key={track.VideoID} track={track} onSelect={setSelected} />
          ))}
        </div>
      )}
      <DetailPanel onRemoved={reload} />
    </div>
  );
};

export { LibraryView };
