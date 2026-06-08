// -- Interfaces ---------------------------------------------------------------

interface SettingRowProps {
  label: string;
  description?: string;
  disabled?: boolean;
  children: React.ReactNode;
}

// -- Constants ----------------------------------------------------------------

const DISABLED_FOOTNOTE = "Wiring lands in a future update.";

// -- Component ----------------------------------------------------------------

const SettingRow: React.FC<SettingRowProps> = ({ label, description, disabled, children }) => (
  <div className="flex flex-col gap-2 py-3">
    <div className="flex items-start justify-between gap-6">
      <div className="flex max-w-md flex-col gap-0.5">
        <span className="text-sm font-medium text-composer-text">{label}</span>
        {description && <span className="text-xs text-composer-text-muted">{description}</span>}
        {disabled && (
          <span className="text-xs text-composer-text-faint italic">{DISABLED_FOOTNOTE}</span>
        )}
      </div>
      <div className="flex shrink-0 items-center">{children}</div>
    </div>
  </div>
);

// -- Exports ------------------------------------------------------------------

export { SettingRow, DISABLED_FOOTNOTE };
