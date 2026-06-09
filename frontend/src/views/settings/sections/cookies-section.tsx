import { useEffect, useRef, useState } from "react";
import { IconChevronRight } from "@tabler/icons-react";
import {
  CookiesState,
  RemoveCookies,
  SetCookiesEnabled,
  SetPreferPremiumAudio,
  UploadCookies,
  VerifyCookies,
} from "../../../../wailsjs/go/app/App";
import type { app, ytdlp } from "../../../../wailsjs/go/models";
import { Button } from "@/components/button";
import { SettingRow } from "@/components/setting-row";
import { TextInput } from "@/components/text-input";
import { Toggle } from "@/components/toggle";
import { cn } from "@/utils/cn";

// -- Helpers ------------------------------------------------------------------

const statusText = (state: app.CookiesStatus | null): string => {
  if (!state) {
    return "Checking…";
  }
  if (!state.present) {
    return "No cookies uploaded. yt-dlp runs anonymously.";
  }
  if (state.enabled) {
    return "Active. The bridge passes your cookies to yt-dlp.";
  }
  return "Uploaded but paused. Toggle to enable.";
};

const verifyToneClass = (result: ytdlp.VerifyResult): string => {
  if (!result.loaded) {
    return "text-composer-error-text";
  }
  if (result.authenticated) {
    return "text-composer-text";
  }
  return "text-composer-text-muted";
};

// -- Upload Area --------------------------------------------------------------

interface UploadAreaProps {
  draft: string;
  setDraft: (value: string) => void;
  busy: boolean;
  uploadError: string | null;
  onFile: (file: File) => void;
  onPasteUpload: () => void;
}

const UploadArea: React.FC<UploadAreaProps> = ({
  draft,
  setDraft,
  busy,
  uploadError,
  onFile,
  onPasteUpload,
}) => {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  return (
    <div className="py-3">
      <div
        onClick={() => fileInputRef.current?.click()}
        onDrop={(e) => {
          e.preventDefault();
          if (busy) return;
          const file = e.dataTransfer.files[0];
          if (file) onFile(file);
        }}
        onDragOver={(e) => e.preventDefault()}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            fileInputRef.current?.click();
          }
        }}
        className={cn(
          "flex cursor-pointer flex-col items-center justify-center gap-1 rounded-md border-2 border-dashed border-composer-border bg-composer-input px-4 py-6 text-xs text-composer-text-muted transition-colors",
          "hover:border-composer-accent hover:text-composer-text",
          busy && "pointer-events-none opacity-50",
        )}
        aria-label="Upload cookies file"
      >
        <span className="font-medium text-composer-text">Drop your cookies file here</span>
        <span>or click to browse (.txt or .json)</span>
      </div>
      <input
        ref={fileInputRef}
        type="file"
        accept=".txt,.json,text/plain,application/json"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) {
            onFile(file);
            e.target.value = "";
          }
        }}
      />
      {uploadError && (
        <p className="mt-2 text-xs text-composer-error-text select-text">{uploadError}</p>
      )}
      <details className="group mt-3">
        <summary className="flex cursor-pointer list-none items-center gap-1 text-xs text-composer-text-muted select-none hover:text-composer-text [&::-webkit-details-marker]:hidden">
          <IconChevronRight size={12} className="transition-transform group-open:rotate-90" />
          Or paste contents
        </summary>
        <div className="mt-2 flex flex-col gap-2">
          <TextInput
            value={draft}
            onChange={setDraft}
            multiline
            rows={5}
            mono
            placeholder="# Netscape HTTP Cookie File or [{...}]"
            ariaLabel="Cookies file contents"
          />
          <div className="flex justify-end">
            <Button
              variant="primary"
              size="sm"
              disabled={busy || !draft.trim()}
              onClick={onPasteUpload}
            >
              {busy ? "Uploading…" : "Upload"}
            </Button>
          </div>
        </div>
      </details>
    </div>
  );
};

// -- Component ----------------------------------------------------------------

const CookiesSection: React.FC = () => {
  const [state, setState] = useState<app.CookiesStatus | null>(null);
  const [draft, setDraft] = useState("");
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [verification, setVerification] = useState<ytdlp.VerifyResult | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = () => {
    CookiesState()
      .then(setState)
      .catch((err: unknown) => console.error("CookiesState failed", err));
  };

  useEffect(refresh, []);

  const handleUpload = async () => {
    if (!draft.trim()) {
      return;
    }
    setBusy(true);
    setUploadError(null);
    try {
      await UploadCookies(draft);
      setDraft("");
      setVerification(null);
      refresh();
    } catch (err: unknown) {
      setUploadError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const handleFile = async (file: File) => {
    setUploadError(null);
    setBusy(true);
    try {
      const content = await file.text();
      await UploadCookies(content);
      setDraft("");
      setVerification(null);
      refresh();
    } catch (err: unknown) {
      setUploadError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const handleRemove = async () => {
    setBusy(true);
    try {
      await RemoveCookies();
      setVerification(null);
      refresh();
    } catch (err: unknown) {
      console.error("RemoveCookies failed", err);
    } finally {
      setBusy(false);
    }
  };

  const handleToggle = async (enabled: boolean) => {
    setBusy(true);
    try {
      await SetCookiesEnabled(enabled);
      refresh();
    } catch (err: unknown) {
      console.error("SetCookiesEnabled failed", err);
    } finally {
      setBusy(false);
    }
  };

  const handlePreferPremium = async (enabled: boolean) => {
    setBusy(true);
    try {
      await SetPreferPremiumAudio(enabled);
      refresh();
    } catch (err: unknown) {
      console.error("SetPreferPremiumAudio failed", err);
    } finally {
      setBusy(false);
    }
  };

  const handleVerify = async () => {
    setBusy(true);
    try {
      const result = await VerifyCookies();
      setVerification(result);
    } catch (err: unknown) {
      setVerification({
        loaded: false,
        authenticated: false,
        rotated: false,
        detail: err instanceof Error ? err.message : String(err),
      } as ytdlp.VerifyResult);
    } finally {
      setBusy(false);
    }
  };

  const present = state?.present ?? false;
  const enabled = state?.enabled ?? false;
  const preferPremium = state?.prefer_premium ?? false;

  return (
    <section className="flex flex-col">
      <h2 className="mb-1 text-xs font-medium tracking-wide text-composer-text-muted">
        Cookies
      </h2>
      <div className="mb-3 flex flex-col gap-2 rounded-md border border-composer-border bg-composer-input px-3 py-3 text-xs text-composer-text-muted">
        <div className="flex flex-col gap-1">
          <h3 className="text-xs font-medium text-composer-text">Why upload cookies</h3>
          <p>
            YouTube has been blocking anonymous downloads more aggressively. Giving yt-dlp a
            cookies file from a signed-in browser session lets it act like that session and
            usually clears the bot wall.
          </p>
        </div>
        <div className="flex flex-col gap-1">
          <h3 className="text-xs font-medium text-composer-text">How to export</h3>
          <p>
            Install the "Get cookies.txt LOCALLY", "Cookie-Editor", or any similar extension.
            Open YouTube while signed in, export the cookies, then drop the file below. The
            bridge accepts both Netscape (.txt) and JSON (.json) exports.
          </p>
        </div>
        <div className="flex flex-col gap-1">
          <h3 className="text-xs font-medium text-composer-text">Treat them like a password</h3>
          <p>
            These cookies grant full access to the YouTube account they came from. Do not
            share them, do not commit them. If anything leaks, go to your Google Account
            security page and sign out of all sessions to revoke them.
          </p>
        </div>
        <div className="flex flex-col gap-1">
          <h3 className="text-xs font-medium text-composer-text">Where they live</h3>
          <p>
            The bridge writes the file to
            {" "}
            <span className="select-text font-mono text-composer-text-secondary">
              {state?.path || "~/.composer-bridge/cookies.txt"}
            </span>
            . Nothing leaves your machine. Composer in the browser never sees it.
          </p>
        </div>
      </div>
      <div className="divide-y divide-composer-border">
        <SettingRow label="Status" description={statusText(state)}>
          {present && (
            <Toggle
              checked={enabled}
              disabled={busy}
              onChange={(v) => {
                void handleToggle(v);
              }}
              ariaLabel="Cookies enabled"
            />
          )}
        </SettingRow>
        <UploadArea
          draft={draft}
          setDraft={setDraft}
          busy={busy}
          uploadError={uploadError}
          onFile={(file) => {
            void handleFile(file);
          }}
          onPasteUpload={() => {
            void handleUpload();
          }}
        />
        {present && (
          <SettingRow
            label="Verify cookies"
            description="Runs a quick yt-dlp probe to check the session is still valid."
          >
            <Button
              variant="secondary"
              size="sm"
              disabled={busy}
              onClick={() => {
                void handleVerify();
              }}
            >
              {busy ? "Verifying…" : "Verify now"}
            </Button>
          </SettingRow>
        )}
        {present && enabled && (
          <SettingRow
            label="Prefer Premium audio quality"
            description="Tries YouTube Music's higher quality tier first when your cookies have a Premium sign-in. The probe stalls for 30 seconds or more per request and falls back to the standard tier when Premium is not available. Leave off if downloads feel slow."
          >
            <Toggle
              checked={preferPremium}
              disabled={busy}
              onChange={(v) => {
                void handlePreferPremium(v);
              }}
              ariaLabel="Prefer Premium audio quality"
            />
          </SettingRow>
        )}
        {verification && (
          <p className={`py-2 text-xs select-text ${verifyToneClass(verification)}`}>
            {verification.detail || (verification.authenticated ? "Cookies look good." : "")}
            {verification.rotated && (
              <span className="ml-1 text-composer-text-muted">
                (YouTube rotated the cookies. Re-export from your browser if this keeps happening.)
              </span>
            )}
          </p>
        )}
        {present && (
          <SettingRow
            label="Remove cookies"
            description="Deletes the file from disk. You can always re-upload later."
          >
            <Button
              variant="destructive"
              size="sm"
              disabled={busy}
              onClick={() => {
                void handleRemove();
              }}
            >
              Remove
            </Button>
          </SettingRow>
        )}
      </div>
    </section>
  );
};

// -- Exports ------------------------------------------------------------------

export { CookiesSection };
