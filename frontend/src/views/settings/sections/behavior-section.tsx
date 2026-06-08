import { SettingRow } from "@/components/setting-row";
import { Toggle } from "@/components/toggle";
import { NumberInput } from "@/components/number-input";
import { Select } from "@/components/select";
import type { config } from "../../../../wailsjs/go/models";

// -- Interfaces ----------------------------------------------------------------

interface BehaviorSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Components ----------------------------------------------------------------

const BehaviorSection: React.FC<BehaviorSectionProps> = ({ config, update }) => {
  return (
    <section className="flex flex-col">
      <h2 className="mb-2 text-sm font-semibold text-text">Behavior</h2>
      <SettingRow
        label="Open at login"
        description="Start the bridge automatically when you log in"
        disabled
      >
        <Toggle
          checked={config.open_at_login}
          onChange={(v) => update("open_at_login", v)}
          disabled
          ariaLabel="Open at login"
        />
      </SettingRow>
      <SettingRow
        label="Show menu bar icon when idle"
        description="Keep a tray icon visible even with no active activity"
      >
        <Toggle
          checked={config.show_menu_bar_icon}
          onChange={(v) => update("show_menu_bar_icon", v)}
          ariaLabel="Show menu bar icon when idle"
        />
      </SettingRow>
      <SettingRow
        label="Max concurrent downloads"
        description="Upper bound on parallel yt-dlp jobs"
      >
        <NumberInput
          value={config.max_concurrent}
          onChange={(v) => update("max_concurrent", v)}
          min={1}
          max={10}
          ariaLabel="Max concurrent downloads"
        />
      </SettingRow>
      <SettingRow label="Default audio format" description="Preferred container for downloads">
        <Select
          value={config.audio_format || "m4a"}
          onChange={(v) => update("audio_format", v)}
          options={[
            { value: "m4a", label: "m4a" },
            { value: "webm", label: "webm" },
            { value: "mp3", label: "mp3" },
          ]}
          ariaLabel="Default audio format"
        />
      </SettingRow>
      <SettingRow label="Default audio quality">
        <Select
          value={config.audio_quality || "best"}
          onChange={(v) => update("audio_quality", v)}
          options={[
            { value: "best", label: "Best" },
            { value: "high", label: "High" },
            { value: "medium", label: "Medium" },
          ]}
          ariaLabel="Default audio quality"
        />
      </SettingRow>
    </section>
  );
};

export { BehaviorSection };
