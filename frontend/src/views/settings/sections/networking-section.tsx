import type { config } from "../../../../wailsjs/go/models";
import { NumberInput } from "@/components/number-input";
import { OriginListInput } from "@/components/origin-list-input";
import { SettingRow } from "@/components/setting-row";
import { Toggle } from "@/components/toggle";

// -- Interfaces ---------------------------------------------------------------

interface NetworkingSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Component ----------------------------------------------------------------

const NetworkingSection: React.FC<NetworkingSectionProps> = ({ config, update }) => (
  <section className="flex flex-col">
    <h2 className="mb-1 text-xs font-medium tracking-wide text-composer-text-muted">
      Networking
    </h2>
    <div className="divide-y divide-composer-border">
      <SettingRow label="Listen port" description="HTTP port the bridge serves on. Takes effect on next restart.">
        <NumberInput
          value={config.listen_port}
          onChange={(v) => update("listen_port", v)}
          min={1024}
          max={65535}
          ariaLabel="Listen port"
        />
      </SettingRow>
      <SettingRow
        label="Use random port if busy"
        description="Fall back to an ephemeral port when the configured one is taken."
      >
        <Toggle
          checked={config.use_random_if_busy}
          onChange={(v) => update("use_random_if_busy", v)}
          ariaLabel="Use random port if busy"
        />
      </SettingRow>
      <div className="flex flex-col gap-2 py-3">
        <div className="flex max-w-md flex-col gap-0.5">
          <span className="text-sm font-medium text-composer-text">
            Allowed Composer origins
          </span>
          <span className="text-xs text-composer-text-muted">
            Cross-origin requests outside this list are blocked.
          </span>
        </div>
        <OriginListInput
          origins={config.allowed_origins}
          onChange={(next) => update("allowed_origins", next)}
          ariaLabel="Allowed Composer origins"
        />
      </div>
    </div>
  </section>
);

// -- Exports ------------------------------------------------------------------

export { NetworkingSection };
