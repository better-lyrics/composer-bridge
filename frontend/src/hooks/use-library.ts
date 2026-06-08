import { useCallback, useEffect, useMemo, useState } from "react";
import { ListTracks } from "../../wailsjs/go/main/App";
import type { library } from "../../wailsjs/go/models";
import { useUIStore, type LibrarySort } from "@/stores/ui-store";

// -- Helpers -------------------------------------------------------------------

function sortTracks(tracks: library.Track[], sort: LibrarySort): library.Track[] {
  if (sort === "recent") {
    return tracks.toSorted((a, b) => b.ImportedAt - a.ImportedAt);
  }
  if (sort === "title") {
    return tracks.toSorted((a, b) => a.Title.localeCompare(b.Title));
  }
  if (sort === "artist") {
    return tracks.toSorted((a, b) => a.Artist.localeCompare(b.Artist));
  }
  return tracks.toSorted((a, b) => a.DurationSec - b.DurationSec);
}

function filterTracks(tracks: library.Track[], search: string): library.Track[] {
  if (!search.trim()) return tracks;
  const needle = search.trim().toLowerCase();
  return tracks.filter(
    (t) =>
      t.Title.toLowerCase().includes(needle) ||
      t.Artist.toLowerCase().includes(needle) ||
      t.Album.toLowerCase().includes(needle),
  );
}

// -- Public --------------------------------------------------------------------

interface UseLibraryResult {
  tracks: library.Track[];
  reload: () => void;
  loading: boolean;
  error: Error | null;
}

export function useLibrary(): UseLibraryResult {
  const sort = useUIStore((s) => s.librarySort);
  const search = useUIStore((s) => s.librarySearch);
  const [raw, setRaw] = useState<library.Track[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const reload = useCallback(() => {
    setLoading(true);
    ListTracks()
      .then((tracks) => {
        setRaw(tracks ?? []);
        setError(null);
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err : new Error(String(err)));
      })
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const tracks = useMemo(() => sortTracks(filterTracks(raw, search), sort), [raw, search, sort]);
  return { tracks, reload, loading, error };
}
