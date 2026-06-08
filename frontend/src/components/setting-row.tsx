// -- Interfaces ----------------------------------------------------------------

interface SettingRowProps {
  label: string;
  description?: string;
  disabled?: boolean;
  children: React.ReactNode;
}

// -- Constants -----------------------------------------------------------------

const DISABLED_FOOTNOTE = "Wiring lands in a future update.";

// -- Components ----------------------------------------------------------------

const SettingRow: React.FC<SettingRowProps> = ({ label, description, disabled, children }) => {
  return (
    <div className="flex items-start justify-between gap-6 border-b border-border py-3 last:border-b-0">
      <div className="flex max-w-md flex-col gap-0.5">
        <span className="text-sm font-medium text-text">{label}</span>
        {description && <span className="text-xs text-text-muted">{description}</span>}
        {disabled && <span className="text-xs text-text-muted italic">{DISABLED_FOOTNOTE}</span>}
      </div>
      <div className="flex shrink-0 items-center">{children}</div>
    </div>
  );
};

export { SettingRow, DISABLED_FOOTNOTE };
