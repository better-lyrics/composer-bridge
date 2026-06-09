import { IconCircleDot, IconLoader2, IconPlayerStop } from "@tabler/icons-react";
import type React from "react";
import { useUIStore } from "@/stores/ui-store";
import { cn } from "@/utils/cn";

// -- Types --------------------------------------------------------------------

type Server = "stopped" | "starting" | "running" | "stopping";
type Download = "idle" | "active";

interface Visual {
  Icon: typeof IconCircleDot;
  iconClass: string;
  spin: boolean;
  label: string;
}

// -- Helpers ------------------------------------------------------------------

function renderVisual(server: Server, download: Download, videoId: string): Visual {
  if (server === "running" && download === "active") {
    return {
      Icon: IconLoader2,
      iconClass: "text-composer-accent",
      spin: true,
      label: `Downloading ${videoId}`,
    };
  }
  if (server === "running") {
    return {
      Icon: IconCircleDot,
      iconClass: "text-emerald-400",
      spin: false,
      label: "Online",
    };
  }
  if (server === "starting") {
    return {
      Icon: IconLoader2,
      iconClass: "text-composer-text-muted",
      spin: true,
      label: "Starting...",
    };
  }
  if (server === "stopping") {
    return {
      Icon: IconLoader2,
      iconClass: "text-composer-text-muted",
      spin: true,
      label: "Stopping...",
    };
  }
  return {
    Icon: IconPlayerStop,
    iconClass: "text-composer-text-muted",
    spin: false,
    label: "Server stopped",
  };
}

// -- Component ----------------------------------------------------------------

const SidebarStatus: React.FC = () => {
  const status = useUIStore((s) => s.bridgeStatus);
  const setView = useUIStore((s) => s.setView);

  if (!status) return null;

  const visual = renderVisual(
    status.server as Server,
    status.download as Download,
    status.downloadVideoId,
  );
  const { Icon } = visual;

  return (
    <button
      type="button"
      onClick={() => setView("settings")}
      className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-xs text-composer-text-secondary transition-colors hover:bg-composer-button/50 hover:text-composer-text"
    >
      <Icon size={14} className={cn(visual.iconClass, visual.spin && "animate-spin")} />
      <span className="truncate">{visual.label}</span>
    </button>
  );
};

// -- Exports ------------------------------------------------------------------

export { SidebarStatus };
