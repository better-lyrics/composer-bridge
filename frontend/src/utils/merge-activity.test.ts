import { describe, expect, it } from "vitest";
import type { activity } from "../../wailsjs/go/models";
import { mergeActivitySnapshot } from "@/utils/merge-activity";

function entry(id: number, status: string, endedAt = 0): activity.Entry {
  return {
    id,
    kind: "audio_download",
    video_id: "RgKAFK5djSk",
    started_at: id * 1000,
    ended_at: endedAt,
    status,
    message: "",
  } as activity.Entry;
}

describe("mergeActivitySnapshot", () => {
  it("regression: a live event newer than the snapshot read is not lost", () => {
    const snapshot = [entry(2, "running"), entry(1, "ok", 1500)];
    const live = [entry(3, "running"), entry(2, "ok", 2600)];

    const merged = mergeActivitySnapshot(snapshot, live);

    expect(merged.map((e) => [e.id, e.status])).toEqual([
      [3, "running"],
      [2, "ok"],
      [1, "ok"],
    ]);
  });

  it("keeps the snapshot row when it is further along than the buffered one", () => {
    const merged = mergeActivitySnapshot([entry(2, "error", 2600)], [entry(2, "running")]);
    expect(merged[0].status).toBe("error");
  });
});
