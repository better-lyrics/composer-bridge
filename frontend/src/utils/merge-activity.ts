import type { activity } from "../../wailsjs/go/models";

// A row has moved further along once it has ended, and later when it ended later.
function progress(e: activity.Entry): number {
  return e.ended_at > 0 ? e.ended_at : -1;
}

// mergeActivitySnapshot combines a RecentActivity read with rows live events
// buffered while that read was in flight. For each id the further-along row
// wins, so an event that lands between the DB read and its response is kept.
export function mergeActivitySnapshot(
  snapshot: activity.Entry[],
  live: activity.Entry[],
): activity.Entry[] {
  const byId = new Map<number, activity.Entry>();
  for (const e of [...snapshot, ...live]) {
    const prev = byId.get(e.id);
    if (!prev || progress(e) > progress(prev)) byId.set(e.id, e);
  }
  return [...byId.values()].sort((a, b) => b.started_at - a.started_at || b.id - a.id);
}
