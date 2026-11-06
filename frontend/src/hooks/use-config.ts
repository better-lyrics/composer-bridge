import { useCallback, useEffect, useRef, useState } from "react";
import { GetConfig, SaveConfig } from "../../wailsjs/go/app/App";
import type { config } from "../../wailsjs/go/models";
import { useDebouncedCallback } from "@/hooks/use-debounced-callback";
import { useUIStore } from "@/stores/ui-store";

// -- Constants ----------------------------------------------------------------

const SAVE_DEBOUNCE_MS = 300;

// -- Public -------------------------------------------------------------------

export type SaveStatus = "idle" | "saving" | "saved" | "error";

interface UseConfigResult {
  config: config.Config | null;
  loaded: boolean;
  loadError: string | null;
  retryLoad: () => void;
  saveStatus: SaveStatus;
  update<K extends keyof config.Config>(key: K, value: config.Config[K]): void;
}

export function useConfig(): UseConfigResult {
  const cfg = useUIStore((s) => s.bridgeConfig);
  const setBridgeConfig = useUIStore((s) => s.setBridgeConfig);
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("idle");
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const cfgRef = useRef<config.Config | null>(cfg);
  cfgRef.current = cfg;

  useEffect(() => {
    if (cfg !== null) return;
    let cancelled = false;
    setLoadError(null);
    GetConfig()
      .then((loaded) => {
        if (cancelled) return;
        setBridgeConfig(loaded);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        console.error("GetConfig failed", err);
        setLoadError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [cfg, setBridgeConfig, loadAttempt]);

  const retryLoad = useCallback(() => setLoadAttempt((n) => n + 1), []);

  const saveSeqRef = useRef(0);
  const persist = useDebouncedCallback((next: config.Config) => {
    setSaveStatus("saving");
    const seq = ++saveSeqRef.current;
    SaveConfig(next)
      .then(() => {
        if (seq === saveSeqRef.current) setSaveStatus("saved");
      })
      .catch((err) => {
        console.error("SaveConfig failed", err);
        if (seq === saveSeqRef.current) setSaveStatus("error");
      });
  }, SAVE_DEBOUNCE_MS);

  const update = useCallback(
    <K extends keyof config.Config>(key: K, value: config.Config[K]) => {
      if (!cfgRef.current) return;
      const next = { ...cfgRef.current, [key]: value } as config.Config;
      cfgRef.current = next;
      setBridgeConfig(next);
      persist(next);
    },
    [persist, setBridgeConfig],
  );

  return { config: cfg, loaded: cfg !== null, loadError, retryLoad, saveStatus, update };
}
