import { SettingRow } from "@/components/setting-row";
import { NumberInput } from "@/components/number-input";
import { TextInput } from "@/components/text-input";
import { Toggle } from "@/components/toggle";
import type { config } from "../../../../wailsjs/go/models";

// -- Interfaces ----------------------------------------------------------------

interface NetworkingSectionProps {
  config: config.Config;
  update: <K extends keyof config.Config>(key: K, value: config.Config[K]) => void;
}

// -- Components ----------------------------------------------------------------

const NetworkingSection: React.FC<NetworkingSectionProps> = ({ config, update }) => {
  return (
    <section className="flex flex-col">
      <h2 className="mb-2 text-sm font-semibold text-text">Networking</h2>
      <SettingRow label="Listen port" description="HTTP port the bridge serves on">
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
        description="If the listen port is taken, fall back to a random free one"
      >
        <Toggle
          checked={config.use_random_if_busy}
          onChange={(v) => update("use_random_if_busy", v)}
          ariaLabel="Use random port if busy"
        />
      </SettingRow>
      <SettingRow
        label="Allowed Composer origins"
        description="One origin per line. Cross-origin requests outside this list are blocked"
      >
        <TextInput
          multiline
          rows={4}
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
          className="w-80"
        />
      </SettingRow>
    </section>
  );
};

export { NetworkingSection };
