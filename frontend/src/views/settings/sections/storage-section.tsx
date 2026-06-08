import { useEffect, useState } from "react";
import { LibrarySize, ThumbCacheSize } from "../../../../wailsjs/go/app/App";
import type { config } from "../../../../wailsjs/go/models";
import { SettingRow } from "@/components/setting-row";
import { TextInput } from "@/components/text-input";

// -- Interfaces ---------------------------------------------------------------

interface StorageSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Helpers ------------------------------------------------------------------

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

// -- Component ----------------------------------------------------------------

const StorageSection: React.FC<StorageSectionProps> = ({ config, update }) => {
  const [librarySize, setLibrarySize] = useState<number | null>(null);
  const [thumbSize, setThumbSize] = useState<number | null>(null);

  useEffect(() => {
    LibrarySize()
      .then(setLibrarySize)
      .catch((err: unknown) => console.error("LibrarySize failed", err));
    ThumbCacheSize()
      .then(setThumbSize)
      .catch((err: unknown) => console.error("ThumbCacheSize failed", err));
  }, []);

  return (
    <section className="flex flex-col">
      <h2 className="mb-1 text-xs font-medium uppercase tracking-wider text-composer-text-muted">
        Storage
      </h2>
      <div className="divide-y divide-composer-border">
        <SettingRow label="Data directory" description="Where library.db, activity.db, and yt-dlp live.">
          <TextInput
            value={config.data_dir}
            onChange={(v) => update("data_dir", v)}
            placeholder="~/.composer-bridge"
            mono
            ariaLabel="Data directory"
            className="w-72"
          />
        </SettingRow>
        <SettingRow label="Library size" description="Total bytes used by downloaded audio.">
          <span className="font-mono text-xs text-composer-text-secondary select-text">
            {librarySize === null ? "…" : formatBytes(librarySize)}
          </span>
        </SettingRow>
        <SettingRow label="Thumbnail cache size" description="Disk used by cached album art.">
          <span className="font-mono text-xs text-composer-text-secondary select-text">
            {thumbSize === null ? "…" : formatBytes(thumbSize)}
          </span>
        </SettingRow>
        <SettingRow
          label="Download location"
          description="Where opt-in audio downloads land. Defaults to ~/Music/Composer Bridge."
        >
          <TextInput
            value={config.download_dir}
            onChange={(v) => update("download_dir", v)}
            placeholder="~/Music/Composer Bridge"
            mono
            ariaLabel="Download location"
            className="w-72"
          />
        </SettingRow>
      </div>
    </section>
  );
};

// -- Exports ------------------------------------------------------------------

export { StorageSection };
