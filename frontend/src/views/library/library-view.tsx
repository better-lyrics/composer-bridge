import { IconExternalLink, IconSearch } from "@tabler/icons-react";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import { Button } from "@/components/button";
import { Select } from "@/components/select";
import { TextInput } from "@/components/text-input";
import { useLibrary } from "@/hooks/use-library";
import { useUIStore, type LibrarySort } from "@/stores/ui-store";
import { DetailPanel } from "@/views/library/detail-panel";
import { TrackCard } from "@/views/library/track-card";

// -- Constants ----------------------------------------------------------------

const COMPOSER_URL = "https://composer.boidu.dev";

const SORT_OPTIONS: { value: LibrarySort; label: string }[] = [
  { value: "recent", label: "Recently imported" },
  { value: "title", label: "Title A-Z" },
  { value: "artist", label: "Artist A-Z" },
  { value: "duration", label: "Duration" },
];

// -- Sub-components -----------------------------------------------------------

const EmptyState: React.FC = () => (
  <div className="flex flex-1 flex-col items-center justify-center gap-3 py-20 text-center">
    <p className="text-lg text-composer-text-secondary">No tracks yet.</p>
    <p className="max-w-sm text-sm text-composer-text-muted">
      Open Composer, enable the Composer Bridge experiment in Advanced settings, then import any
      YouTube link. Tracks land here automatically.
    </p>
    <Button variant="secondary" size="sm" hasIcon onClick={() => BrowserOpenURL(COMPOSER_URL)}>
      <IconExternalLink size={14} />
      Open Composer
    </Button>
  </div>
);

// -- View ---------------------------------------------------------------------

const LibraryView: React.FC = () => {
  const { tracks, reload } = useLibrary();
  const sort = useUIStore((s) => s.librarySort);
  const setSort = useUIStore((s) => s.setLibrarySort);
  const search = useUIStore((s) => s.librarySearch);
  const setSearch = useUIStore((s) => s.setLibrarySearch);
  const setSelected = useUIStore((s) => s.setSelectedVideoId);

  return (
    <div className="flex h-full flex-col">
      <header className="sticky top-0 z-10 flex items-center justify-between gap-4 border-b border-composer-border bg-composer-bg px-6 py-4">
        <div className="flex flex-col gap-0.5">
          <h1 className="text-xl font-semibold tracking-tight text-composer-text">Library</h1>
          <span className="text-xs text-composer-text-muted">
            {tracks.length} {tracks.length === 1 ? "track" : "tracks"}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <IconSearch
              size={14}
              className="absolute top-1/2 left-2 -translate-y-1/2 text-composer-text-muted pointer-events-none"
            />
            <TextInput
              value={search}
              onChange={setSearch}
              placeholder="Search title, artist, album"
              ariaLabel="Search library"
              className="w-64 pl-7"
            />
          </div>
          <Select value={sort} onChange={setSort} options={SORT_OPTIONS} ariaLabel="Sort tracks" />
        </div>
      </header>
      <div className="flex-1 overflow-auto px-6 py-6">
        {tracks.length === 0 ? (
          <EmptyState />
        ) : (
          <div
            className="grid gap-4"
            style={{ gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))" }}
          >
            {tracks.map((track) => (
              <TrackCard key={track.video_id} track={track} onSelect={setSelected} />
            ))}
          </div>
        )}
      </div>
      <DetailPanel onRemoved={reload} />
    </div>
  );
};

// -- Exports ------------------------------------------------------------------

export { LibraryView };
