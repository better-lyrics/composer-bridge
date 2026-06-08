import { SettingRow } from "@/components/setting-row";
import { Select } from "@/components/select";
import { TextInput } from "@/components/text-input";
import { cn } from "@/utils/cn";
import type { config } from "../../../../wailsjs/go/models";

// -- Interfaces ----------------------------------------------------------------

interface YtdlpSectionProps {
  config: config.Config;
  bridgeVersion: string;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Components ----------------------------------------------------------------

const YtdlpSection: React.FC<YtdlpSectionProps> = ({ config, bridgeVersion, update }) => {
  return (
    <section className="flex flex-col">
      <h2 className="mb-2 text-sm font-semibold text-text">yt-dlp</h2>
      <SettingRow
        label="Update channel"
        description="Which yt-dlp release stream to follow"
      >
        <Select
          value={config.ytdlp_channel || "stable"}
          onChange={(v) => update("ytdlp_channel", v)}
          options={[
            { value: "stable", label: "Stable" },
            { value: "nightly", label: "Nightly" },
            { value: "off", label: "Off" },
          ]}
          ariaLabel="yt-dlp update channel"
        />
      </SettingRow>
      <SettingRow
        label="Binary path override"
        description="Leave blank to use the bundled binary"
      >
        <TextInput
          value={config.ytdlp_binary_path}
          onChange={(v) => update("ytdlp_binary_path", v)}
          placeholder="auto-detect"
          ariaLabel="yt-dlp binary path"
          className="w-64"
        />
      </SettingRow>
      <SettingRow
        label="Force update now"
        description="Trigger an immediate yt-dlp update"
        disabled
      >
        <button
          type="button"
          disabled
          title="Wiring lands in a future update."
          className={cn(
            "rounded-md border border-border bg-surface-elevated px-3 py-1.5 text-sm",
            "text-text-muted cursor-not-allowed opacity-60",
          )}
        >
          Update now
        </button>
      </SettingRow>
      <SettingRow label="Bridge version">
        <span className="text-sm text-text-muted select-text">{bridgeVersion || "Loading..."}</span>
      </SettingRow>
      <SettingRow label="yt-dlp version" disabled>
        <span className="text-sm text-text-muted">Loading...</span>
      </SettingRow>
    </section>
  );
};

export { YtdlpSection };
