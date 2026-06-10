import { IconActivity, IconMusic, IconSettings } from "@tabler/icons-react";
import { m } from "motion/react";
import { BetterLyricsLogo } from "@/components/better-lyrics-logo";
import { SidebarStatus } from "@/components/sidebar-status";
import { useUIStore, type View } from "@/stores/ui-store";
import { cn } from "@/utils/cn";

// Spring tuned for ~180ms effective settle with no overshoot, the iOS-feeling
// reflow profile the user asked for. Shared by every layout-animated child of
// the sidebar column so the header, nav, and status footer all move together
// when the banner mounts or unmounts above them.
const LAYOUT_TRANSITION = {
  layout: { type: "spring" as const, stiffness: 500, damping: 45 },
};

// -- Constants ----------------------------------------------------------------

interface NavItem {
  id: View;
  label: string;
  Icon: typeof IconMusic;
}

const NAV_ITEMS: NavItem[] = [
  { id: "library", label: "Library", Icon: IconMusic },
  { id: "activity", label: "Activity", Icon: IconActivity },
  { id: "settings", label: "Settings", Icon: IconSettings },
];

interface SidebarProps {
  // bannerVisible signals that the update banner is mounted above the app
  // shell, so it owns the macOS title-bar inset zone. When true the sidebar
  // skips its own large top padding to avoid pushing the logo well past the
  // visible chrome.
  bannerVisible?: boolean;
}

// -- Component ----------------------------------------------------------------

const Sidebar: React.FC<SidebarProps> = ({ bannerVisible = false }) => {
  const view = useUIStore((s) => s.view);
  const setView = useUIStore((s) => s.setView);

  return (
    <aside className="flex h-full w-52 shrink-0 flex-col border-r border-composer-border bg-composer-bg select-none">
      <m.div
        layout
        transition={LAYOUT_TRANSITION}
        className={cn(
          "flex items-center gap-2 px-4 pb-4",
          bannerVisible ? "pt-4" : "pt-14",
        )}
        style={{ "--wails-draggable": "drag" } as React.CSSProperties}
      >
        <BetterLyricsLogo size={20} />
        <span className="text-sm font-semibold tracking-tight text-composer-text">
          Composer Bridge
        </span>
      </m.div>
      <m.nav
        layout
        transition={LAYOUT_TRANSITION}
        className="flex flex-1 flex-col gap-1 px-2 py-2"
        style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
      >
        {NAV_ITEMS.map(({ id, label, Icon }) => {
          const active = view === id;
          return (
            <button
              key={id}
              type="button"
              onClick={() => setView(id)}
              aria-current={active ? "page" : undefined}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-left transition-colors",
                active
                  ? "bg-composer-button text-composer-text font-medium"
                  : "text-composer-text-secondary hover:bg-composer-button/50 hover:text-composer-text",
              )}
            >
              <Icon size={16} />
              <span>{label}</span>
            </button>
          );
        })}
      </m.nav>
      <m.div
        layout
        transition={LAYOUT_TRANSITION}
        className="mt-auto px-2 pb-3"
        style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
      >
        <SidebarStatus />
      </m.div>
    </aside>
  );
};

// -- Exports ------------------------------------------------------------------

export { Sidebar };
