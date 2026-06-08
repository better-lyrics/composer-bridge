import { SettingRow } from "@/components/setting-row";
import { Select } from "@/components/select";
import { cn } from "@/utils/cn";
import type { config } from "../../../../wailsjs/go/models";

// -- Interfaces ----------------------------------------------------------------

interface DiagnosticsSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Constants -----------------------------------------------------------------

const DISABLED_BTN_CLASSES = cn(
  "rounded-md border border-border bg-surface-elevated px-3 py-1.5 text-sm",
  "text-text-muted cursor-not-allowed opacity-60",
);

// -- Components ----------------------------------------------------------------

const DiagnosticsSection: React.FC<DiagnosticsSectionProps> = ({ config, update }) => {
  return (
    <section className="flex flex-col">
      <h2 className="mb-2 text-sm font-semibold text-text">Diagnostics</h2>
      <SettingRow label="Log level" description="Verbosity of bridge logs">
        <Select
          value={config.log_level || "info"}
          onChange={(v) => update("log_level", v)}
          options={[
            { value: "debug", label: "Debug" },
            { value: "info", label: "Info" },
            { value: "warn", label: "Warn" },
            { value: "error", label: "Error" },
          ]}
          ariaLabel="Log level"
        />
      </SettingRow>
      <SettingRow label="Open log file" description="Reveal the current log" disabled>
        <button type="button" disabled className={DISABLED_BTN_CLASSES}>
          Open log
        </button>
      </SettingRow>
      <SettingRow
        label="Copy diagnostic report"
        description="Capture a redacted bundle for bug reports"
        disabled
      >
        <button type="button" disabled className={DISABLED_BTN_CLASSES}>
          Copy report
        </button>
      </SettingRow>
    </section>
  );
};

export { DiagnosticsSection };
