import { IconActivity, IconMusic, IconSettings } from "@tabler/icons-react";
import { useUIStore, type View } from "@/stores/ui-store";
import { cn } from "@/utils/cn";
import { BetterLyricsLogo } from "@/components/better-lyrics-logo";

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

const Sidebar: React.FC = () => {
  const view = useUIStore((s) => s.view);
  const setView = useUIStore((s) => s.setView);

  return (
    <aside className="flex h-full w-56 shrink-0 flex-col border-r border-border bg-surface select-none">
      <div className="flex items-center gap-2 px-4 py-4 text-bl-red">
        <BetterLyricsLogo size={20} />
        <span className="text-sm font-semibold tracking-tight text-text">Composer Bridge</span>
      </div>
      <nav className="flex flex-1 flex-col gap-1 px-2">
        {NAV_ITEMS.map(({ id, label, Icon }) => {
          const active = view === id;
          return (
            <button
              key={id}
              type="button"
              onClick={() => setView(id)}
              className={cn(
                "flex items-center gap-2 rounded-md px-3 py-2 text-sm cursor-pointer transition-colors",
                active
                  ? "bg-bl-red-soft text-bl-red"
                  : "text-text-muted hover:bg-bl-red-soft hover:text-text",
              )}
              aria-current={active ? "page" : undefined}
            >
              <Icon size={16} />
              <span>{label}</span>
            </button>
          );
        })}
      </nav>
    </aside>
  );
};

export { Sidebar };
