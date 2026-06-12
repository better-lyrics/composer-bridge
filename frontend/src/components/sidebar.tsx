import { IconActivity, IconMusic, IconSettings } from "@tabler/icons-react";
import { BetterLyricsLogo } from "@/components/better-lyrics-logo";
import { SidebarStatus } from "@/components/sidebar-status";
import { useUIStore, type View } from "@/stores/ui-store";
import { cn } from "@/utils/cn";
import { isMacOS } from "@/utils/is-mac";

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
  // bannerVisible only matters on macOS: the sidebar's title-bar inset shrinks
  // while the update banner is mounted so the logo doesn't sit in dead space
  // below the banner. On Windows and Linux the native title bar already insets
  // the webview, so the sidebar uses a single small top padding and the prop
  // is ignored.
  bannerVisible?: boolean;
}

// -- Component ----------------------------------------------------------------

const Sidebar: React.FC<SidebarProps> = ({ bannerVisible = false }) => {
  const view = useUIStore((s) => s.view);
  const setView = useUIStore((s) => s.setView);

  const headerSpacingClass = isMacOS
    ? cn(
        "will-change-[padding-top]",
        "transition-[padding-top] duration-[220ms] ease-[cubic-bezier(0.32,0.72,0,1)]",
        bannerVisible ? "pt-4" : "pt-14",
      )
    : "pt-4";

  return (
    <aside className="flex h-full w-52 shrink-0 flex-col border-r border-composer-border bg-composer-bg select-none">
      <div
        className={cn("flex items-center gap-2 px-4 pb-4", headerSpacingClass)}
        style={{ "--wails-draggable": "drag" } as React.CSSProperties}
      >
        <BetterLyricsLogo size={20} />
        <span className="text-sm font-semibold tracking-tight text-composer-text">
          Composer Bridge
        </span>
      </div>
      <nav
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
      </nav>
      <div
        className="mt-auto px-2 pb-3"
        style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
      >
        <SidebarStatus />
      </div>
    </aside>
  );
};

// -- Exports ------------------------------------------------------------------

export { Sidebar };
