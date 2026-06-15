import { domAnimation, LazyMotion } from "motion/react";
import { Sidebar } from "@/components/sidebar";
import { UpdateBanner } from "@/components/update-banner";
import { useActivityStream } from "@/hooks/use-activity-stream";
import { useBridgeStatus } from "@/hooks/use-bridge-status";
import { useUpdateInfo } from "@/hooks/use-update-info";
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
  useBridgeStatus();
  useActivityStream();
  const { showBanner } = useUpdateInfo();

  return (
    <LazyMotion features={domAnimation} strict>
      <div className="flex h-full flex-col bg-composer-bg text-composer-text">
        <UpdateBanner />
        <div className="flex flex-1 overflow-hidden">
          <Sidebar bannerVisible={showBanner} />
          <main className="flex-1 overflow-auto">
            <ActiveView />
          </main>
        </div>
      </div>
    </LazyMotion>
  );
};

export { App };
