import { Sidebar } from "@/components/sidebar";
import { useUIStore, type View } from "@/stores/ui-store";
import { ActivityView } from "@/views/activity/activity-view";
import { LibraryView } from "@/views/library/library-view";
import { SettingsView } from "@/views/settings/settings-view";

const VIEWS: Record<View, React.FC> = {
  library: LibraryView,
  activity: ActivityView,
  settings: SettingsView,
};

const App: React.FC = () => {
  const view = useUIStore((s) => s.view);
  const ActiveView = VIEWS[view];

  return (
    <div className="flex h-full bg-composer-bg text-composer-text">
      <Sidebar />
      <main className="flex-1 overflow-auto">
        <ActiveView />
      </main>
    </div>
  );
};

export { App };
