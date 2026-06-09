import { IconLoader2, IconPlayerStop } from "@tabler/icons-react";
import type React from "react";
import { useUIStore } from "@/stores/ui-store";
import { cn } from "@/utils/cn";

// -- Types --------------------------------------------------------------------

type Server = "stopped" | "starting" | "running" | "stopping";
type Download = "idle" | "active";

// -- Sub-components -----------------------------------------------------------

const PingDot: React.FC<{ colorClass: string }> = ({ colorClass }) => (
  <span className="relative inline-flex h-2.5 w-2.5 items-center justify-center">
    <span className={cn("absolute inline-flex h-full w-full rounded-full opacity-75 animate-ping", colorClass)} />
    <span className={cn("relative inline-flex h-2 w-2 rounded-full", colorClass)} />
  </span>
);

// -- Component ----------------------------------------------------------------

const SidebarStatus: React.FC = () => {
  const status = useUIStore((s) => s.bridgeStatus);
  const setView = useUIStore((s) => s.setView);

  if (!status) return null;

  const server = status.server as Server;
  const download = status.download as Download;

  let indicator: React.ReactNode;
  let label: string;
  let textClass = "text-composer-text-secondary";

  if (server === "running" && download === "active") {
    indicator = <IconLoader2 size={14} className="animate-spin text-composer-accent" />;
    label = `Downloading ${status.downloadVideoId}`;
  } else if (server === "running") {
    indicator = <PingDot colorClass="bg-emerald-400" />;
    label = "Online";
  } else if (server === "starting") {
    indicator = <IconLoader2 size={14} className="animate-spin text-composer-text-muted" />;
    label = "Starting...";
  } else if (server === "stopping") {
    indicator = <IconLoader2 size={14} className="animate-spin text-composer-text-muted" />;
    label = "Stopping...";
  } else {
    indicator = <IconPlayerStop size={14} className="text-composer-text-muted" />;
    label = "Server stopped";
    textClass = "text-composer-text-muted";
  }

  return (
    <div>
      <div className="flex items-center gap-2 px-3 pt-2 pb-1.5">
        <span className="text-[10px] font-medium uppercase tracking-wider text-composer-text-muted">Status</span>
        <div className="h-px flex-1 bg-composer-border" />
      </div>
      <button
        type="button"
        onClick={() => setView("settings")}
        className={cn(
          "flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-xs transition-colors hover:bg-composer-button/50 hover:text-composer-text",
          textClass,
        )}
      >
        {indicator}
        <span className="truncate">{label}</span>
      </button>
    </div>
  );
};

// -- Exports ------------------------------------------------------------------

export { SidebarStatus };
