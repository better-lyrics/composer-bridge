import { render, screen, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { ActivityView } from "@/views/activity/activity-view";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";

let bindings: AppBindings;

function entry(overrides: Record<string, unknown> = {}) {
  return {
    ID: 1,
    Kind: "audio_download",
    VideoID: "RgKAFK5djSk",
    StartedAt: Date.now() - 5000,
    EndedAt: 0,
    Status: "running",
    Message: "",
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
      entry({ ID: 1, Status: "ok", EndedAt: Date.now() }),
      entry({ ID: 2, Kind: "import", VideoID: "ZEcqHA7dbwM", Status: "ok" }),
    ]);
    render(<ActivityView />);
    await waitFor(() => {
      expect(screen.getAllByTestId("activity-row")).toHaveLength(2);
    });
  });

  it("running entry shows spinning loader", async () => {
    bindings.RecentActivity.mockResolvedValue([entry({ Status: "running" })]);
    render(<ActivityView />);
    await waitFor(() => {
      const row = screen.getByTestId("activity-row");
      expect(row.dataset.status).toBe("running");
      expect(row.querySelector(".animate-spin")).not.toBeNull();
    });
  });

  it("ok entry shows a check icon in bl-red", async () => {
    bindings.RecentActivity.mockResolvedValue([entry({ Status: "ok" })]);
    render(<ActivityView />);
    await waitFor(() => {
      const row = screen.getByTestId("activity-row");
      expect(row.dataset.status).toBe("ok");
      expect(row.querySelector(".text-bl-red")).not.toBeNull();
    });
  });

  it("error entry shows rose-toned icon and surfaces the message", async () => {
    bindings.RecentActivity.mockResolvedValue([
      entry({ Status: "error", Message: "yt-dlp boom" }),
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
