import type { config } from "../../../../wailsjs/go/models";
import { NumberInput } from "@/components/number-input";
import { SettingRow } from "@/components/setting-row";
import { TextInput } from "@/components/text-input";
import { Toggle } from "@/components/toggle";

// -- Interfaces ---------------------------------------------------------------

interface NetworkingSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Component ----------------------------------------------------------------

const NetworkingSection: React.FC<NetworkingSectionProps> = ({ config, update }) => (
  <section className="flex flex-col">
    <h2 className="mb-1 text-xs font-medium uppercase tracking-wider text-composer-text-muted">
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
      <SettingRow
        label="Allowed Composer origins"
        description="One origin per line. Cross-origin requests outside this list are blocked."
      >
        <TextInput
          multiline
          rows={4}
          mono
          value={config.allowed_origins.join("\n")}
          onChange={(v) =>
            update(
              "allowed_origins",
              v
                .split("\n")
                .map((s) => s.trim())
                .filter((s) => s.length > 0),
            )
          }
          ariaLabel="Allowed Composer origins"
          className="w-56"
        />
      </SettingRow>
    </div>
  </section>
);

// -- Exports ------------------------------------------------------------------

export { NetworkingSection };
