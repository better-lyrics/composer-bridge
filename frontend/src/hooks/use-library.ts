import { useCallback, useEffect, useMemo, useState } from "react";
import { ListTracks } from "../../wailsjs/go/app/App";
import type { library } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { useUIStore, type LibrarySort } from "@/stores/ui-store";

const LIBRARY_EVENT = "library:update";

// -- Helpers ------------------------------------------------------------------

function sortTracks(tracks: library.Track[], sort: LibrarySort): library.Track[] {
  if (sort === "recent") {
    return tracks.toSorted((a, b) => b.imported_at - a.imported_at);
  }
  if (sort === "title") {
    return tracks.toSorted((a, b) => a.title.localeCompare(b.title));
  }
  if (sort === "artist") {
    return tracks.toSorted((a, b) => a.artist.localeCompare(b.artist));
  }
  return tracks.toSorted((a, b) => a.duration_sec - b.duration_sec);
}

function filterTracks(tracks: library.Track[], search: string): library.Track[] {
  if (!search.trim()) return tracks;
  const needle = search.trim().toLowerCase();
  return tracks.filter(
    (t) =>
      t.title.toLowerCase().includes(needle) ||
      t.artist.toLowerCase().includes(needle) ||
      t.album.toLowerCase().includes(needle),
  );
}

// -- Public -------------------------------------------------------------------

interface UseLibraryResult {
  tracks: library.Track[];
  reload: () => void;
  loaded: boolean;
  loading: boolean;
  error: Error | null;
}

export function useLibrary(): UseLibraryResult {
  const sort = useUIStore((s) => s.librarySort);
  const search = useUIStore((s) => s.librarySearch);
  const raw = useUIStore((s) => s.libraryTracks);
  const setRaw = useUIStore((s) => s.setLibraryTracks);
  const [loading, setLoading] = useState(raw === null);
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
  }, [setRaw]);

  useEffect(() => {
    if (raw === null) reload();
  }, [raw, reload]);

  useEffect(() => {
    let off: (() => void) | undefined;
    try {
      off = EventsOn(LIBRARY_EVENT, () => reload());
    } catch (err) {
      console.error("EventsOn library:update failed", err);
    }
    return () => {
      if (off) off();
    };
  }, [reload]);

  const tracks = useMemo(
    () => sortTracks(filterTracks(raw ?? [], search), sort),
    [raw, search, sort],
  );
  return { tracks, reload, loaded: raw !== null, loading, error };
}
