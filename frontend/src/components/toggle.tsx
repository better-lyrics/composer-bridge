import { cn } from "@/utils/cn";

// -- Interfaces ---------------------------------------------------------------

interface ToggleProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  ariaLabel?: string;
}

// -- Component ----------------------------------------------------------------

const Toggle: React.FC<ToggleProps> = ({ checked, onChange, disabled, ariaLabel }) => (
  <button
    type="button"
    role="switch"
    aria-checked={checked}
    aria-label={ariaLabel}
    disabled={disabled}
    onClick={() => onChange(!checked)}
    className={cn(
      "relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full transition-colors",
      checked ? "bg-composer-accent" : "bg-composer-button",
      disabled && "cursor-not-allowed opacity-40",
    )}
  >
    <span
      className={cn(
        "pointer-events-none inline-block size-4 rounded-full bg-white shadow transform transition-transform mt-0.5",
        checked ? "translate-x-4.5" : "translate-x-0.5",
      )}
    />
  </button>
);

// -- Exports ------------------------------------------------------------------

export { Toggle };
