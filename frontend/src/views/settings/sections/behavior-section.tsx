import { useEffect, useState } from "react";
import { SupportsAutostart } from "../../../../wailsjs/go/app/App";
import type { config } from "../../../../wailsjs/go/models";
import { NumberInput } from "@/components/number-input";
import { Select } from "@/components/select";
import { SettingRow } from "@/components/setting-row";
import { Toggle } from "@/components/toggle";

// -- Interfaces ---------------------------------------------------------------

interface BehaviorSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Component ----------------------------------------------------------------

const BehaviorSection: React.FC<BehaviorSectionProps> = ({ config, update }) => {
  const [autostartSupported, setAutostartSupported] = useState(true);
  useEffect(() => {
    SupportsAutostart()
      .then(setAutostartSupported)
      .catch((err: unknown) => console.error("SupportsAutostart failed", err));
  }, []);

  return (
    <section className="flex flex-col">
      <h2 className="mb-1 text-xs font-medium tracking-wide text-composer-text-muted">
        Behavior
      </h2>
      <div className="divide-y divide-composer-border">
        <SettingRow
          label="Open at login"
          description={
            autostartSupported
              ? "Start the bridge automatically when you sign in."
              : "Auto-start is only supported on macOS for now."
          }
        >
          <Toggle
            checked={config.open_at_login}
            onChange={(v) => update("open_at_login", v)}
            disabled={!autostartSupported}
            ariaLabel="Open at login"
          />
        </SettingRow>
        <SettingRow
          label="Show menu bar icon when idle"
          description="Keep the tray icon visible even when no work is in flight."
        >
          <Toggle
            checked={config.show_menu_bar_icon}
            onChange={(v) => update("show_menu_bar_icon", v)}
            ariaLabel="Show menu bar icon when idle"
          />
        </SettingRow>
        <SettingRow
          label="Max concurrent downloads"
          description="Upper bound on parallel yt-dlp jobs."
        >
          <NumberInput
            value={config.max_concurrent}
            onChange={(v) => update("max_concurrent", v)}
            min={1}
            max={10}
            ariaLabel="Max concurrent downloads"
          />
        </SettingRow>
        <SettingRow label="Default audio format" description="Codec preference for downloads.">
          <Select
            value={config.audio_format || "opus"}
            onChange={(v) => update("audio_format", v)}
            options={[
              { value: "opus", label: "opus" },
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
      </div>
    </section>
  );
};

// -- Exports ------------------------------------------------------------------

export { BehaviorSection };
