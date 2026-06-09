import { IconActivity, IconMusic, IconSettings } from "@tabler/icons-react";
import { BetterLyricsLogo } from "@/components/better-lyrics-logo";
import { SidebarStatus } from "@/components/sidebar-status";
import { useUIStore, type View } from "@/stores/ui-store";
import { cn } from "@/utils/cn";

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

// -- Component ----------------------------------------------------------------

const Sidebar: React.FC = () => {
  const view = useUIStore((s) => s.view);
  const setView = useUIStore((s) => s.setView);

  return (
    <aside className="flex h-full w-52 shrink-0 flex-col border-r border-composer-border bg-composer-bg select-none">
      <div
        className="flex items-center gap-2 px-4 pt-14 pb-4"
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
