import { useEffect, useState } from "react";
import { BridgeVersion, YtdlpVersion } from "../../../wailsjs/go/app/App";
import { useConfig } from "@/hooks/use-config";
import { BehaviorSection } from "@/views/settings/sections/behavior-section";
import { CookiesSection } from "@/views/settings/sections/cookies-section";
import { DiagnosticsSection } from "@/views/settings/sections/diagnostics-section";
import { NetworkingSection } from "@/views/settings/sections/networking-section";
import { ServerSection } from "@/views/settings/sections/server-section";
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
  const { config, loaded, saveStatus, update } = useConfig();
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

  if (!loaded || !config) {
    return (
      <div className="flex h-full flex-col">
        <header className="border-b border-composer-border px-6 py-4">
          <h1 className="text-xl font-semibold tracking-tight text-composer-text">Settings</h1>
        </header>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <header
        className="sticky top-0 z-10 flex items-center justify-between border-b border-composer-border bg-composer-bg px-6 py-4"
        style={{ "--wails-draggable": "drag" } as React.CSSProperties}
      >
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
          <ServerSection config={config} />
          <NetworkingSection config={config} update={update} />
          <YtdlpSection
            config={config}
            bridgeVersion={bridgeVersion}
            ytdlpVersion={ytdlpVersion}
            update={update}
          />
          <CookiesSection />
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
