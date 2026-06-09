import { StartServer, StopServer } from "../../../../wailsjs/go/app/App";
import type { config } from "../../../../wailsjs/go/models";
import { SettingRow } from "@/components/setting-row";
import { Toggle } from "@/components/toggle";
import { useUIStore } from "@/stores/ui-store";

// -- Interfaces ---------------------------------------------------------------

interface ServerSectionProps {
  config: config.Config;
}

// -- Component ----------------------------------------------------------------

const ServerSection: React.FC<ServerSectionProps> = ({ config }) => {
  const status = useUIStore((s) => s.bridgeStatus);
  const server = status?.server ?? "stopped";
  const transitioning = server === "starting" || server === "stopping";
  const checked = server === "running";

  let description: string;
  if (transitioning) {
    description = "Updating...";
  } else if (server === "running") {
    const fallbackHint = config.use_random_if_busy
      ? " (falls back to the random port written in port.txt if the port is busy)"
      : "";
    description = `Listening on http://localhost:${config.listen_port}.${fallbackHint}`;
  } else {
    description = "Composer can't reach the bridge while this is off.";
  }

  return (
    <section className="flex flex-col">
      <h2 className="mb-1 text-xs font-medium tracking-wide text-composer-text-muted">
        Server
      </h2>
      <div className="divide-y divide-composer-border">
        <SettingRow label="Bridge server" description={description}>
          <Toggle
            checked={checked}
            disabled={transitioning}
            onChange={(v) => {
              const promise = v ? StartServer() : StopServer();
              void promise.catch((err: unknown) => {
                console.error(v ? "StartServer failed" : "StopServer failed", err);
              });
            }}
            ariaLabel="Bridge server"
          />
        </SettingRow>
      </div>
    </section>
  );
};

// -- Exports ------------------------------------------------------------------

export { ServerSection };
