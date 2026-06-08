import { useCallback, useEffect, useRef, useState } from "react";
import { GetConfig, SaveConfig } from "../../wailsjs/go/main/App";
import type { config } from "../../wailsjs/go/models";
import { useDebouncedCallback } from "@/hooks/use-debounced-callback";

// -- Constants -----------------------------------------------------------------

const SAVE_DEBOUNCE_MS = 300;

// -- Public --------------------------------------------------------------------

export type SaveStatus = "idle" | "saving" | "saved" | "error";

interface UseConfigResult {
  config: config.Config | null;
  loading: boolean;
  saveStatus: SaveStatus;
  update<K extends keyof config.Config>(key: K, value: config.Config[K]): void;
}

export function useConfig(): UseConfigResult {
  const [cfg, setCfg] = useState<config.Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("idle");
  const cfgRef = useRef<config.Config | null>(null);

  useEffect(() => {
    let cancelled = false;
    GetConfig()
      .then((loaded) => {
        if (cancelled) return;
        cfgRef.current = loaded;
        setCfg(loaded);
      })
      .catch((err) => {
        if (!cancelled) console.error("GetConfig failed", err);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const persist = useDebouncedCallback((next: config.Config) => {
    setSaveStatus("saving");
    SaveConfig(next)
      .then(() => setSaveStatus("saved"))
      .catch((err) => {
        console.error("SaveConfig failed", err);
        setSaveStatus("error");
      });
  }, SAVE_DEBOUNCE_MS);

  const update = useCallback(
    <K extends keyof config.Config>(key: K, value: config.Config[K]) => {
      if (!cfgRef.current) return;
      const next = { ...cfgRef.current, [key]: value } as config.Config;
      cfgRef.current = next;
      setCfg(next);
      persist(next);
    },
    [persist],
  );

  return { config: cfg, loading, saveStatus, update };
}
