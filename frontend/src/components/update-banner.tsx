import { IconArrowUp, IconDownload, IconX } from "@tabler/icons-react";
import { useState } from "react";
import { Button } from "@/components/button";
import { useUpdateInfo } from "@/hooks/use-update-info";

// -- Component ----------------------------------------------------------------

const UpdateBanner: React.FC = () => {
  const { info, showBanner, installing, installError, install, dismissForSession } = useUpdateInfo();
  const [showNotes, setShowNotes] = useState(false);

  if (!showBanner || !info) return null;

  return (
    <div
      role="region"
      aria-label="Update available"
      className="flex flex-col border-b border-composer-border bg-composer-accent-dark/10"
      style={{ "--wails-draggable": "drag" } as React.CSSProperties}
    >
      <div
        className="flex h-13 items-center gap-3 pr-4 pl-24"
        style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
      >
        <div className="flex flex-1 items-center gap-2 select-none">
          <IconArrowUp size={14} className="text-composer-accent" />
          <span className="text-xs font-medium text-composer-text">
            Update available
          </span>
          <span className="font-mono text-[11px] text-composer-text-muted select-text">
            v{info.latest}
          </span>
          {info.notes && (
            <button
              type="button"
              onClick={() => setShowNotes((s) => !s)}
              className="cursor-pointer text-[11px] text-composer-text-muted underline-offset-2 hover:text-composer-text hover:underline"
            >
              {showNotes ? "Hide notes" : "What's new"}
            </button>
          )}
          {installError && (
            <span className="text-[11px] text-composer-error-text select-text">
              {installError}
            </span>
          )}
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={dismissForSession}
          disabled={installing}
        >
          Later
        </Button>
        <Button
          variant="primary"
          size="sm"
          hasIcon
          onClick={install}
          disabled={installing}
        >
          <IconDownload size={12} />
          {installing ? "Installing..." : "Install"}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={dismissForSession}
          disabled={installing}
          aria-label="Close"
        >
          <IconX size={14} />
        </Button>
      </div>
      {showNotes && info.notes && (
        <div className="border-t border-composer-border px-4 py-3">
          <pre className="font-mono text-[11px] whitespace-pre-wrap text-composer-text-muted select-text">
            {info.notes}
          </pre>
        </div>
      )}
    </div>
  );
};

// -- Exports ------------------------------------------------------------------

export { UpdateBanner };
