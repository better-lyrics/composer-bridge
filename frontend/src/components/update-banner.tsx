import { IconArrowUp, IconDownload, IconX } from "@tabler/icons-react";
import { AnimatePresence, m } from "motion/react";
import { useState } from "react";
import { Button } from "@/components/button";
import { useUpdateInfo } from "@/hooks/use-update-info";

// -- Constants ----------------------------------------------------------------

// Snappy spring that settles in roughly 180ms with no overshoot. Layout
// reflow uses the same profile as the sidebar so the banner mount and the
// sidebar shift play in lockstep through the shared LayoutGroup.
const LAYOUT_SPRING = { type: "spring" as const, stiffness: 500, damping: 45 };
const BANNER_TRANSITION = {
  layout: LAYOUT_SPRING,
  opacity: { duration: 0.16, ease: [0.32, 0.72, 0, 1] as const },
};
const NOTES_TRANSITION = {
  layout: LAYOUT_SPRING,
  opacity: { duration: 0.14, ease: [0.32, 0.72, 0, 1] as const },
};

// -- Component ----------------------------------------------------------------

const UpdateBanner: React.FC = () => {
  const { info, showBanner, installing, installError, install, dismissForSession } = useUpdateInfo();
  const [showNotes, setShowNotes] = useState(false);

  return (
    <AnimatePresence initial={false}>
      {showBanner && info && (
        <m.div
          key="update-banner"
          layout
          role="region"
          aria-label="Update available"
          className="flex flex-col overflow-hidden border-b border-composer-border bg-composer-accent-dark/10"
          style={{ "--wails-draggable": "drag" } as React.CSSProperties}
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: "auto", opacity: 1 }}
          exit={{ height: 0, opacity: 0 }}
          transition={BANNER_TRANSITION}
        >
          <div
            className="flex h-13 items-center gap-3 pr-4 pl-24"
            style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
          >
            <div className="flex flex-1 items-center gap-2 select-none">
              <IconArrowUp size={14} className="text-composer-accent" />
              <span className="text-xs font-medium text-composer-text">Update available</span>
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
              size="sm"
              onClick={dismissForSession}
              disabled={installing}
              aria-label="Close"
              className="size-7 p-0"
            >
              <IconX size={14} />
            </Button>
          </div>
          <AnimatePresence initial={false}>
            {showNotes && info.notes && (
              <m.div
                key="update-notes"
                layout
                className="overflow-hidden border-t border-composer-border"
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: "auto", opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={NOTES_TRANSITION}
              >
                <pre className="px-4 py-3 font-mono text-[11px] whitespace-pre-wrap text-composer-text-muted select-text">
                  {info.notes}
                </pre>
              </m.div>
            )}
          </AnimatePresence>
        </m.div>
      )}
    </AnimatePresence>
  );
};

// -- Exports ------------------------------------------------------------------

export { UpdateBanner };
