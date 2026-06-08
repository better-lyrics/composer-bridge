import { render, screen, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { ActivityView } from "@/views/activity/activity-view";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";

let bindings: AppBindings;

function entry(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    kind: "audio_download",
    video_id: "RgKAFK5djSk",
    started_at: Date.now() - 5000,
    ended_at: 0,
    status: "running",
    message: "",
    ...overrides,
  };
}

beforeEach(() => {
  bindings = setupWailsMock();
});

afterEach(() => {
  cleanup();
  resetWailsMock();
});

describe("ActivityView", () => {
  it("renders a row for each entry", async () => {
    bindings.RecentActivity.mockResolvedValue([
      entry({ id: 1, status: "ok", ended_at: Date.now() }),
      entry({ id: 2, kind: "import", video_id: "ZEcqHA7dbwM", status: "ok" }),
    ]);
    render(<ActivityView />);
    await waitFor(() => {
      expect(screen.getAllByTestId("activity-row")).toHaveLength(2);
    });
  });

  it("running entry shows spinning loader", async () => {
    bindings.RecentActivity.mockResolvedValue([entry({ status: "running" })]);
    render(<ActivityView />);
    await waitFor(() => {
      const row = screen.getByTestId("activity-row");
      expect(row.dataset.status).toBe("running");
      expect(row.querySelector(".animate-spin")).not.toBeNull();
    });
  });

  it("ok entry shows a check icon in bl-red", async () => {
    bindings.RecentActivity.mockResolvedValue([entry({ status: "ok" })]);
    render(<ActivityView />);
    await waitFor(() => {
      const row = screen.getByTestId("activity-row");
      expect(row.dataset.status).toBe("ok");
      expect(row.querySelector(".text-bl-red")).not.toBeNull();
    });
  });

  it("error entry shows rose-toned icon and surfaces the message", async () => {
    bindings.RecentActivity.mockResolvedValue([
      entry({ status: "error", message: "yt-dlp boom" }),
    ]);
    render(<ActivityView />);
    await waitFor(() => {
      const row = screen.getByTestId("activity-row");
      expect(row.dataset.status).toBe("error");
      expect(row.querySelector(".text-rose-500")).not.toBeNull();
      expect(screen.getByText(/yt-dlp boom/i)).toBeInTheDocument();
    });
  });
});
