// -- Constants -----------------------------------------------------------------

const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3600;
const MS_PER_SECOND = 1000;
const SECONDS_IN_MINUTE_THRESHOLD = 60;
const SECONDS_IN_HOUR_THRESHOLD = 3600;
const SECONDS_IN_DAY_THRESHOLD = 86_400;

// -- Helpers -------------------------------------------------------------------

function pad(value: number): string {
  return value.toString().padStart(2, "0");
}

// -- Public --------------------------------------------------------------------

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "0:00";
  }
  const whole = Math.floor(seconds);
  const hours = Math.floor(whole / SECONDS_PER_HOUR);
  const minutes = Math.floor((whole % SECONDS_PER_HOUR) / SECONDS_PER_MINUTE);
  const remainingSeconds = whole % SECONDS_PER_MINUTE;
  if (hours > 0) {
    return `${hours}:${pad(minutes)}:${pad(remainingSeconds)}`;
  }
  return `${minutes}:${pad(remainingSeconds)}`;
}

// formatRelativeTime returns a coarse "5s ago" style string for recent
// timestamps and an absolute Mar 5, 14:32 style for older ones. nowEpochMs is
// injectable so tests don't need to freeze Date.now globally.
export function formatRelativeTime(epochMs: number, nowEpochMs: number = Date.now()): string {
  const diffSeconds = Math.max(0, Math.floor((nowEpochMs - epochMs) / MS_PER_SECOND));
  if (diffSeconds < SECONDS_IN_MINUTE_THRESHOLD) {
    return `${diffSeconds}s ago`;
  }
  if (diffSeconds < SECONDS_IN_HOUR_THRESHOLD) {
    return `${Math.floor(diffSeconds / SECONDS_PER_MINUTE)}m ago`;
  }
  if (diffSeconds < SECONDS_IN_DAY_THRESHOLD) {
    return `${Math.floor(diffSeconds / SECONDS_PER_HOUR)}h ago`;
  }
  const date = new Date(epochMs);
  const month = date.toLocaleString("en-US", { month: "short" });
  const day = date.getDate();
  const hours = pad(date.getHours());
  const minutes = pad(date.getMinutes());
  return `${month} ${day}, ${hours}:${minutes}`;
}
