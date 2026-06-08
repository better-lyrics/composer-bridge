import { useState } from "react";
import {
  BuildDiagnosticReport,
  OpenLogFile,
} from "../../../../wailsjs/go/app/App";
import { BrowserOpenURL } from "../../../../wailsjs/runtime/runtime";
import type { config } from "../../../../wailsjs/go/models";
import { Button } from "@/components/button";
import { Select } from "@/components/select";
import { SettingRow } from "@/components/setting-row";

// -- Interfaces ---------------------------------------------------------------

interface DiagnosticsSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Component ----------------------------------------------------------------

const DiagnosticsSection: React.FC<DiagnosticsSectionProps> = ({ config, update }) => {
  const [copied, setCopied] = useState(false);

  const openLog = async () => {
    try {
      const url = await OpenLogFile();
      BrowserOpenURL(url);
    } catch (err: unknown) {
      console.error("OpenLogFile failed", err);
    }
  };

  const copyReport = async () => {
    try {
      const report = await BuildDiagnosticReport();
      await navigator.clipboard.writeText(report);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch (err: unknown) {
      console.error("BuildDiagnosticReport failed", err);
    }
  };

  return (
    <section className="flex flex-col">
      <h2 className="mb-1 text-xs font-medium tracking-wide text-composer-text-muted">
        Diagnostics
      </h2>
      <div className="divide-y divide-composer-border">
        <SettingRow label="Log level" description="Verbosity of bridge logs.">
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
        <SettingRow label="Open log file" description="Reveal the bridge.log file in your system viewer.">
          <Button variant="secondary" size="sm" onClick={openLog}>
            Open log
          </Button>
        </SettingRow>
        <SettingRow
          label="Copy diagnostic report"
          description="Versions, config, and recent activity. Paste into a bug report."
        >
          <Button variant="secondary" size="sm" onClick={copyReport}>
            {copied ? "Copied" : "Copy report"}
          </Button>
        </SettingRow>
      </div>
    </section>
  );
};

// -- Exports ------------------------------------------------------------------

export { DiagnosticsSection };
