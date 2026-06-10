import { Button } from "@/components/button";
import { SettingRow } from "@/components/setting-row";
import { useUpdateInfo } from "@/hooks/use-update-info";

// -- Interfaces ---------------------------------------------------------------

interface UpdatesSectionProps {
  bridgeVersion: string;
}

// -- Component ----------------------------------------------------------------

const UpdatesSection: React.FC<UpdatesSectionProps> = ({ bridgeVersion }) => {
  const { info, checking, installing, checkError, installError, checkNow, install, previewBanner } =
    useUpdateInfo();

  const statusLabel = (() => {
    if (checking) return "Checking…";
    if (checkError) return checkError;
    if (info === null) return "Click to check";
    if (info.available) return `Update available · v${info.latest}`;
    return `Up to date · v${info.current}`;
  })();

  return (
    <section className="flex flex-col">
      <h2 className="mb-1 text-xs font-medium tracking-wide text-composer-text-muted">
        Updates
      </h2>
      <div className="divide-y divide-composer-border">
        <SettingRow label="Current version" description="The bridge version currently running.">
          <span className="font-mono text-xs text-composer-text-muted select-text">
            v{bridgeVersion || "?"}
          </span>
        </SettingRow>
        <SettingRow
          label="Check for updates"
          description="Manually fetch the release manifest. Otherwise polled once every 24 hours."
        >
          <div className="flex items-center gap-2">
            <span
              aria-live="polite"
              className="font-mono text-[11px] text-composer-text-muted select-text"
            >
              {statusLabel}
            </span>
            <Button variant="secondary" size="sm" onClick={checkNow} disabled={checking}>
              {checking ? "Checking…" : "Check now"}
            </Button>
          </div>
        </SettingRow>
        {info?.available && (
          <SettingRow
            label="Install update"
            description="Downloads, verifies, swaps the binary, and relaunches the bridge."
          >
            <div className="flex items-center gap-2">
              {installError && (
                <span className="font-mono text-[11px] text-composer-error-text select-text">
                  {installError}
                </span>
              )}
              <Button variant="primary" size="sm" onClick={install} disabled={installing}>
                {installing ? "Installing…" : `Install v${info.latest}`}
              </Button>
            </div>
          </SettingRow>
        )}
        {import.meta.env.DEV && (
          <SettingRow
            label="Preview update banner (dev only)"
            description="Inject a fake update so the banner renders without a real poll. Visible only in vite dev mode."
          >
            <Button variant="ghost" size="sm" onClick={previewBanner}>
              Show preview
            </Button>
          </SettingRow>
        )}
      </div>
    </section>
  );
};

// -- Exports ------------------------------------------------------------------

export { UpdatesSection };
