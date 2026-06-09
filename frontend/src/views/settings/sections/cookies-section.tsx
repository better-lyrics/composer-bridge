import { useEffect, useState } from "react";
import {
  CookiesState,
  RemoveCookies,
  SetCookiesEnabled,
  UploadCookies,
  VerifyCookies,
} from "../../../../wailsjs/go/app/App";
import type { app, ytdlp } from "../../../../wailsjs/go/models";
import { Button } from "@/components/button";
import { SettingRow } from "@/components/setting-row";
import { TextInput } from "@/components/text-input";
import { Toggle } from "@/components/toggle";

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

  const handleVerify = async () => {
    setBusy(true);
    try {
      const result = await VerifyCookies(undefined);
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
            Install the "Get cookies.txt LOCALLY" extension. Open YouTube while signed in,
            click the extension, pick Export As, choose the Netscape format, then paste the
            file's contents below.
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
        <SettingRow
          label="Paste cookies.txt"
          description="Open the exported file in a text editor and paste the full contents."
        >
          <Button
            variant="primary"
            size="sm"
            disabled={busy || !draft.trim()}
            onClick={() => {
              void handleUpload();
            }}
          >
            {busy ? "Uploading…" : "Upload"}
          </Button>
        </SettingRow>
        <div className="py-3">
          <TextInput
            value={draft}
            onChange={setDraft}
            multiline
            rows={5}
            mono
            placeholder="# Netscape HTTP Cookie File"
            ariaLabel="Cookies file contents"
          />
          {uploadError && (
            <p className="mt-2 text-xs text-composer-error-text">{uploadError}</p>
          )}
        </div>
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
        {verification && (
          <p className={`py-2 text-xs ${verifyToneClass(verification)}`}>
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
