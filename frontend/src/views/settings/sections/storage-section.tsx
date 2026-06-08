import { SettingRow } from "@/components/setting-row";
import { TextInput } from "@/components/text-input";
import type { config } from "../../../../wailsjs/go/models";

// -- Interfaces ----------------------------------------------------------------

interface StorageSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Components ----------------------------------------------------------------

const StorageSection: React.FC<StorageSectionProps> = ({ config, update }) => {
  return (
    <section className="flex flex-col">
      <h2 className="mb-2 text-sm font-semibold text-text">Storage</h2>
      <SettingRow label="Data directory" description="Where library.db, activity.db, and yt-dlp live">
        <TextInput
          value={config.data_dir}
          onChange={(v) => update("data_dir", v)}
          placeholder="~/.composer-bridge"
          ariaLabel="Data directory"
          className="w-72"
        />
      </SettingRow>
      <SettingRow
        label="Library size"
        description="Total size of audio files on disk"
        disabled
      >
        <span className="text-sm text-text-muted">unknown</span>
      </SettingRow>
      <SettingRow
        label="Thumbnail cache size"
        description="Disk used by cached album art"
        disabled
      >
        <span className="text-sm text-text-muted">unknown</span>
      </SettingRow>
      <SettingRow
        label="Default download location"
        description="Where opt-in audio downloads land"
      >
        <TextInput
          value={config.download_dir}
          onChange={(v) => update("download_dir", v)}
          placeholder="~/Music"
          ariaLabel="Default download location"
          className="w-72"
        />
      </SettingRow>
    </section>
  );
};

export { StorageSection };
