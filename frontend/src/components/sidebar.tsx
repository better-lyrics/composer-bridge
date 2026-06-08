import { IconActivity, IconMusic, IconSettings } from "@tabler/icons-react";
import { BetterLyricsLogo } from "@/components/better-lyrics-logo";
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
      <div className="flex items-center gap-2 px-4 py-4">
        <BetterLyricsLogo size={20} />
        <span className="text-sm font-semibold tracking-tight text-composer-text">Composer Bridge</span>
      </div>
      <nav className="flex flex-1 flex-col gap-1 px-2 py-2">
        {NAV_ITEMS.map(({ id, label, Icon }) => {
          const active = view === id;
          return (
            <button
              key={id}
              type="button"
              onClick={() => setView(id)}
              aria-current={active ? "page" : undefined}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-left cursor-pointer transition-colors",
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
      <div className="px-4 py-3 border-t border-composer-border text-[10px] uppercase tracking-wider text-composer-text-faint">
        Bridge
      </div>
    </aside>
  );
};

// -- Exports ------------------------------------------------------------------

export { Sidebar };
