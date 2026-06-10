import { domAnimation, LayoutGroup, LazyMotion } from "motion/react";
import { Sidebar } from "@/components/sidebar";
import { UpdateBanner } from "@/components/update-banner";
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
  const { showBanner } = useUpdateInfo();

  return (
    <LazyMotion features={domAnimation} strict>
      {/* LayoutGroup wraps every element that participates in the banner-mount
          reflow so they share one snapshot cycle. Without it the sidebar
          header would snap to its new position while the banner is still
          tweening, producing the "jump" the user was seeing. */}
      <LayoutGroup>
        <div className="flex h-full flex-col bg-composer-bg text-composer-text">
          <UpdateBanner />
          <div className="flex flex-1 overflow-hidden">
            <Sidebar bannerVisible={showBanner} />
            <main className="flex-1 overflow-auto">
              <ActiveView />
            </main>
          </div>
        </div>
      </LayoutGroup>
    </LazyMotion>
  );
};

export { App };
