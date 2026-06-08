import { useEffect, useState } from "react";
import { BridgeVersion } from "../../../wailsjs/go/main/App";
import { useConfig } from "@/hooks/use-config";
import { NetworkingSection } from "@/views/settings/sections/networking-section";
import { YtdlpSection } from "@/views/settings/sections/ytdlp-section";
import { StorageSection } from "@/views/settings/sections/storage-section";
import { BehaviorSection } from "@/views/settings/sections/behavior-section";
import { DiagnosticsSection } from "@/views/settings/sections/diagnostics-section";

// -- Constants -----------------------------------------------------------------

const STATUS_LABELS: Record<string, string> = {
  idle: "",
  saving: "Saving...",
  saved: "Saved",
  error: "Save failed",
};

// -- Components ----------------------------------------------------------------

const SettingsView: React.FC = () => {
  const { config, loading, saveStatus, update } = useConfig();
  const [version, setVersion] = useState("");

  useEffect(() => {
    BridgeVersion()
      .then(setVersion)
      .catch((err) => console.error("BridgeVersion failed", err));
  }, []);

  if (loading || !config) {
    return (
      <div className="flex flex-col gap-2">
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <p className="text-sm text-text-muted">Loading...</p>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col gap-6">
      <header className="flex items-center justify-between">
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <span
          aria-live="polite"
          className="text-xs text-text-muted"
          data-testid="save-status"
        >
          {STATUS_LABELS[saveStatus]}
        </span>
      </header>
      <div className="flex max-w-3xl flex-col gap-8">
        <NetworkingSection config={config} update={update} />
        <YtdlpSection config={config} bridgeVersion={version} update={update} />
        <StorageSection config={config} update={update} />
        <BehaviorSection config={config} update={update} />
        <DiagnosticsSection config={config} update={update} />
      </div>
    </div>
  );
};

export { SettingsView };
