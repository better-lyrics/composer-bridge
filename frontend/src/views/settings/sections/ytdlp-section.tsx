import { useEffect, useState } from "react";
import { ForceYtdlpUpdate, YtdlpVersion } from "../../../../wailsjs/go/app/App";
import type { config } from "../../../../wailsjs/go/models";
import { Button } from "@/components/button";
import { Select } from "@/components/select";
import { SettingRow } from "@/components/setting-row";
import { TextInput } from "@/components/text-input";

// -- Interfaces ---------------------------------------------------------------

interface YtdlpSectionProps {
  config: config.Config;
  bridgeVersion: string;
  ytdlpVersion: string;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Component ----------------------------------------------------------------

const YtdlpSection: React.FC<YtdlpSectionProps> = ({
  config,
  bridgeVersion,
  ytdlpVersion,
  update,
}) => {
  const [updating, setUpdating] = useState(false);
  const [updateError, setUpdateError] = useState<string | null>(null);
  const [localVersion, setLocalVersion] = useState(ytdlpVersion);

  useEffect(() => {
    setLocalVersion(ytdlpVersion);
  }, [ytdlpVersion]);

  const handleForceUpdate = async () => {
    setUpdating(true);
    setUpdateError(null);
    try {
      const next = await ForceYtdlpUpdate();
      setLocalVersion(next);
    } catch (err: unknown) {
      setUpdateError(err instanceof Error ? err.message : String(err));
      try {
        setLocalVersion(await YtdlpVersion());
      } catch {
        // best-effort refresh; surface the original error instead.
      }
    } finally {
      setUpdating(false);
    }
  };

  const effectiveYtdlp = localVersion || ytdlpVersion;

  return (
    <section className="flex flex-col">
      <h2 className="mb-1 text-xs font-medium tracking-wide text-composer-text-muted">
        yt-dlp
      </h2>
      <div className="divide-y divide-composer-border">
        <SettingRow label="Update channel" description="Which yt-dlp release stream to follow">
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
          description="Leave blank to use the bundled binary."
        >
          <TextInput
            value={config.ytdlp_binary_path}
            onChange={(v) => update("ytdlp_binary_path", v)}
            placeholder="auto-detect"
            mono
            ariaLabel="yt-dlp binary path"
            className="w-48"
          />
        </SettingRow>
        <SettingRow
          label="Force update now"
          description="Re-download the latest yt-dlp release."
        >
          <Button variant="secondary" size="sm" disabled={updating} onClick={handleForceUpdate}>
            {updating ? "Updating…" : "Update now"}
          </Button>
        </SettingRow>
        {updateError && (
          <p className="py-2 text-xs text-composer-error-text">{updateError}</p>
        )}
        <SettingRow label="Bridge version">
          <span className="font-mono text-xs text-composer-text-secondary select-text">
            {bridgeVersion || "Loading…"}
          </span>
        </SettingRow>
        <SettingRow label="yt-dlp version">
          <span className="font-mono text-xs text-composer-text-secondary select-text">
            {effectiveYtdlp || "Loading…"}
          </span>
        </SettingRow>
      </div>
    </section>
  );
};

// -- Exports ------------------------------------------------------------------

export { YtdlpSection };
