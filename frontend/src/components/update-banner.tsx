import { IconDownload, IconSquareRoundedArrowUp, IconX } from "@tabler/icons-react";
import { AnimatePresence, m } from "motion/react";
import { useState } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { Button } from "@/components/button";
import { useUpdateInfo } from "@/hooks/use-update-info";

// -- Constants ----------------------------------------------------------------

// 220ms with the iOS-native ease-out-quart curve. The height tween on the
// banner naturally pushes the flex column below it each frame, so the sidebar
// reflows for free without needing motion's layout system. The notes section
// uses a slightly faster tween to feel responsive on toggle.
const BANNER_TRANSITION = { duration: 0.22, ease: [0.32, 0.72, 0, 1] as const };
const NOTES_TRANSITION = { duration: 0.18, ease: [0.32, 0.72, 0, 1] as const };

// INLINE_TOKEN_RE splits a line into runs of plain text, **bold** spans, and
// http(s) URLs while keeping the matched delimiters. The capturing group
// inside String.prototype.split is what lets us reconstruct the line with
// tokens still in place. Bold is greedy across `*` to handle the common
// "**Full diff:** ..." shape without false-matching `*` in prose.
const INLINE_TOKEN_RE = /(\*\*[^*]+\*\*|https?:\/\/\S+)/g;

// -- Notes rendering ----------------------------------------------------------

// renderInline parses a single line's inline tokens: **bold** turns into a
// brighter span, http(s) URLs into clickable anchors that route through
// Wails' BrowserOpenURL (the default browser, not the embedded WebView).
function renderInline(text: string): React.ReactNode[] {
  const tokens = text.split(INLINE_TOKEN_RE);
  return tokens.map((tok, i) => {
    if (!tok) return null;
    if (tok.startsWith("**") && tok.endsWith("**")) {
      return (
        <strong key={i} className="font-semibold">
          {tok.slice(2, -2)}
        </strong>
      );
    }
    if (tok.startsWith("http://") || tok.startsWith("https://")) {
      return (
        <a
          key={i}
          href={tok}
          onClick={(e) => {
            e.preventDefault();
            BrowserOpenURL(tok);
          }}
          className="text-composer-accent-text hover:underline"
        >
          {tok}
        </a>
      );
    }
    return <span key={i}>{tok}</span>;
  });
}

// renderNotes does the line-level layout: headings get color + weight (no
// size change, per the user spec), bullets get a dot prefix, blank source
// lines are dropped. Vertical rhythm is driven by the parent's `gap-2` (8px
// between every block) plus an extra `mt-2` above headings so they read as
// section starts rather than another bullet.
function renderNotes(notes: string): React.ReactNode[] {
  const lines = notes.split("\n");
  return lines.map((rawLine, i) => {
    const line = rawLine.trimEnd();
    if (line === "") return null;
    if (line.startsWith("### ")) {
      return (
        <div key={i} className="mt-2 font-semibold text-composer-text-secondary first:mt-0">
          {renderInline(line.slice(4))}
        </div>
      );
    }
    if (line.startsWith("## ")) {
      return (
        <div key={i} className="mt-2 font-semibold text-composer-text first:mt-0">
          {renderInline(line.slice(3))}
        </div>
      );
    }
    if (line.startsWith("# ")) {
      return (
        <div key={i} className="mt-2 font-semibold text-composer-text first:mt-0">
          {renderInline(line.slice(2))}
        </div>
      );
    }
    if (line.startsWith("- ") || line.startsWith("* ")) {
      return (
        <div key={i} className="flex gap-2 pl-1">
          <span className="text-composer-text-faint" aria-hidden>
            •
          </span>
          <span className="flex-1">{renderInline(line.slice(2))}</span>
        </div>
      );
    }
    return <div key={i}>{renderInline(line)}</div>;
  });
}

// -- Component ----------------------------------------------------------------

const UpdateBanner: React.FC = () => {
  const { info, showBanner, installing, installError, install, dismissForSession } = useUpdateInfo();
  const [showNotes, setShowNotes] = useState(false);

  return (
    <AnimatePresence initial={false}>
      {showBanner && info && (
        <m.div
          key="update-banner"
          role="region"
          aria-label="Update available"
          className="flex flex-col overflow-hidden border-b border-composer-border bg-composer-accent-dark/10 will-change-[height,opacity]"
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
              <IconSquareRoundedArrowUp size={16} className="text-composer-accent" />
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
                className="overflow-hidden border-t border-composer-border will-change-[height,opacity]"
                style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: "auto", opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={NOTES_TRANSITION}
              >
                <div className="flex cursor-text flex-col gap-2 px-4 py-3 text-[11px] leading-relaxed text-composer-text-muted select-text">
                  {renderNotes(info.notes)}
                </div>
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
