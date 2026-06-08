import { useEffect, useState } from "react";
import { BridgeVersion, YtdlpVersion } from "../../../wailsjs/go/app/App";
import { useConfig } from "@/hooks/use-config";
import { BehaviorSection } from "@/views/settings/sections/behavior-section";
import { DiagnosticsSection } from "@/views/settings/sections/diagnostics-section";
import { NetworkingSection } from "@/views/settings/sections/networking-section";
import { StorageSection } from "@/views/settings/sections/storage-section";
import { YtdlpSection } from "@/views/settings/sections/ytdlp-section";

// -- Constants ----------------------------------------------------------------

const STATUS_LABELS: Record<string, string> = {
  idle: "",
  saving: "Saving…",
  saved: "Saved",
  error: "Save failed",
};

// -- View ---------------------------------------------------------------------

const SettingsView: React.FC = () => {
  const { config, loading, saveStatus, update } = useConfig();
  const [bridgeVersion, setBridgeVersion] = useState("");
  const [ytdlpVersion, setYtdlpVersion] = useState("");

  useEffect(() => {
    BridgeVersion()
      .then(setBridgeVersion)
      .catch((err: unknown) => console.error("BridgeVersion failed", err));
    YtdlpVersion()
      .then(setYtdlpVersion)
      .catch((err: unknown) => console.error("YtdlpVersion failed", err));
  }, []);

  if (loading || !config) {
    return (
      <div className="flex h-full flex-col">
        <header className="border-b border-composer-border px-6 py-4">
          <h1 className="text-xl font-semibold tracking-tight text-composer-text">Settings</h1>
        </header>
        <div className="px-6 py-6 text-sm text-composer-text-muted">Loading…</div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <header className="sticky top-0 z-10 flex items-center justify-between border-b border-composer-border bg-composer-bg px-6 py-4">
        <div className="flex flex-col gap-0.5">
          <h1 className="text-xl font-semibold tracking-tight text-composer-text">Settings</h1>
          <span className="text-xs text-composer-text-muted">Saved automatically</span>
        </div>
        <span
          aria-live="polite"
          className="font-mono text-[11px] text-composer-text-muted"
          data-testid="save-status"
        >
          {STATUS_LABELS[saveStatus]}
        </span>
      </header>
      <div className="flex-1 overflow-auto px-6 py-6">
        <div className="flex max-w-3xl flex-col gap-8">
          <NetworkingSection config={config} update={update} />
          <YtdlpSection
            config={config}
            bridgeVersion={bridgeVersion}
            ytdlpVersion={ytdlpVersion}
            update={update}
          />
          <StorageSection config={config} update={update} />
          <BehaviorSection config={config} update={update} />
          <DiagnosticsSection config={config} update={update} />
        </div>
      </div>
    </div>
  );
};

// -- Exports ------------------------------------------------------------------

export { SettingsView };
