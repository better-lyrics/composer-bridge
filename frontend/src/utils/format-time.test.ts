import { describe, it, expect } from "vitest";
import { formatDuration, formatRelativeTime } from "@/utils/format-time";

describe("formatDuration", () => {
  it("zero seconds renders 0:00", () => {
    expect(formatDuration(0)).toBe("0:00");
  });

  it("under an hour renders mm:ss with zero-padded seconds", () => {
    expect(formatDuration(65)).toBe("1:05");
    expect(formatDuration(60)).toBe("1:00");
    expect(formatDuration(9)).toBe("0:09");
  });

  it("an hour and above renders h:mm:ss", () => {
    expect(formatDuration(3725)).toBe("1:02:05");
    expect(formatDuration(3600)).toBe("1:00:00");
  });

  it("negative or non-finite collapse to 0:00", () => {
    expect(formatDuration(-1)).toBe("0:00");
    expect(formatDuration(Number.NaN)).toBe("0:00");
    expect(formatDuration(Number.POSITIVE_INFINITY)).toBe("0:00");
  });
});

describe("formatRelativeTime", () => {
  const NOW = 1_700_000_000_000;

  it("less than a minute renders seconds", () => {
    expect(formatRelativeTime(NOW - 5_000, NOW)).toBe("5s ago");
    expect(formatRelativeTime(NOW, NOW)).toBe("0s ago");
  });

  it("under an hour renders minutes", () => {
    expect(formatRelativeTime(NOW - 2 * 60_000, NOW)).toBe("2m ago");
  });

  it("under a day renders hours", () => {
    expect(formatRelativeTime(NOW - 60 * 60_000, NOW)).toBe("1h ago");
  });

  it("older falls back to an absolute date string", () => {
    const twoDaysAgo = NOW - 2 * 86_400_000;
    const out = formatRelativeTime(twoDaysAgo, NOW);
    expect(out).toMatch(/^[A-Z][a-z]{2} \d+, \d{2}:\d{2}$/);
  });
});
