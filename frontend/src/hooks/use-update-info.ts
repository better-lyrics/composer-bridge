import { useCallback, useEffect, useState } from "react";
import { CheckForUpdates, InstallUpdate, LatestUpdate } from "../../wailsjs/go/app/App";
import type { updater } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { useUIStore } from "@/stores/ui-store";

// -- Constants ----------------------------------------------------------------

const EVENT_NAME = "bridge:update-available";
const IGNORED_VERSION_KEY = "composer-bridge:update-banner-ignored-version";

// -- Public -------------------------------------------------------------------

interface UseUpdateInfoResult {
  info: updater.UpdateInfo | null;
  showBanner: boolean;
  checking: boolean;
  installing: boolean;
  checkError: string | null;
  installError: string | null;
  checkNow: () => Promise<void>;
  install: () => Promise<void>;
  dismissForSession: () => void;
  ignoreThisVersion: () => void;
  // previewBanner injects a fake UpdateInfo so the banner can be inspected
  // without waiting for a poll or running a real check. Only callable from
  // dev builds (gated by import.meta.env.DEV at the call site).
  previewBanner: () => void;
}

function readIgnoredVersion(): string | null {
  try {
    return window.localStorage.getItem(IGNORED_VERSION_KEY);
  } catch {
    return null;
  }
}

function writeIgnoredVersion(version: string): void {
  try {
    window.localStorage.setItem(IGNORED_VERSION_KEY, version);
  } catch {
    // localStorage write can fail in private mode; the banner will reappear
    // next session, which is acceptable.
  }
}

export function useUpdateInfo(): UseUpdateInfoResult {
  const info = useUIStore((s) => s.updateInfo);
  const setInfo = useUIStore((s) => s.setUpdateInfo);
  const dismissed = useUIStore((s) => s.updateBannerDismissed);
  const setDismissed = useUIStore((s) => s.setUpdateBannerDismissed);
  const [checking, setChecking] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [checkError, setCheckError] = useState<string | null>(null);
  const [installError, setInstallError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    LatestUpdate()
      .then((next) => {
        if (!cancelled) setInfo(next ?? null);
      })
      .catch((err: unknown) => {
        console.error("LatestUpdate failed", err);
      });
    return () => {
      cancelled = true;
    };
  }, [setInfo]);

  useEffect(() => {
    let off: (() => void) | undefined;
    try {
      off = EventsOn(EVENT_NAME, (next: updater.UpdateInfo) => {
        setInfo(next ?? null);
        // A fresh detection clears any session dismissal so the user sees
        // the new version's banner even if they dismissed the prior poll.
        setDismissed(false);
      });
    } catch (err) {
      console.error("EventsOn failed", err);
    }
    return () => {
      if (off) off();
    };
  }, [setInfo, setDismissed]);

  const checkNow = useCallback(async () => {
    setChecking(true);
    setCheckError(null);
    try {
      const next = await CheckForUpdates();
      setInfo(next ?? null);
      setDismissed(false);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setCheckError(message);
    } finally {
      setChecking(false);
    }
  }, [setInfo, setDismissed]);

  const install = useCallback(async () => {
    setInstalling(true);
    setInstallError(null);
    try {
      await InstallUpdate();
      // On success, the bridge process is on its way out and Wails will
      // tear down this window. Nothing to clean up here.
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setInstallError(message);
      setInstalling(false);
    }
  }, []);

  const dismissForSession = useCallback(() => {
    setDismissed(true);
  }, [setDismissed]);

  const ignoreThisVersion = useCallback(() => {
    if (info?.latest) {
      writeIgnoredVersion(info.latest);
    }
    setDismissed(true);
  }, [info, setDismissed]);

  const previewBanner = useCallback(() => {
    const fake = {
      available: true,
      current: "0.0.0",
      latest: "99.99.99-preview",
      released_at: new Date().toISOString(),
      notes: "Preview mode: this is a fake update info injected for UI inspection.",
      asset: { url: "", sha256: "" },
    } as updater.UpdateInfo;
    setInfo(fake);
    setDismissed(false);
  }, [setInfo, setDismissed]);

  const ignoredVersion = readIgnoredVersion();
  const showBanner =
    !!info &&
    info.available &&
    !dismissed &&
    (ignoredVersion === null || ignoredVersion !== info.latest);

  return {
    info,
    showBanner,
    checking,
    installing,
    checkError,
    installError,
    checkNow,
    install,
    dismissForSession,
    ignoreThisVersion,
    previewBanner,
  };
}
