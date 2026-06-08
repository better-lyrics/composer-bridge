import { cn } from "@/utils/cn";

// -- Interfaces ---------------------------------------------------------------

interface SelectOption<T extends string> {
  value: T;
  label: string;
}

interface SelectProps<T extends string> {
  value: T;
  onChange: (value: T) => void;
  options: SelectOption<T>[];
  disabled?: boolean;
  ariaLabel?: string;
}

// -- Component ----------------------------------------------------------------

function Select<T extends string>({
  value,
  onChange,
  options,
  disabled,
  ariaLabel,
}: SelectProps<T>): React.ReactElement {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value as T)}
      disabled={disabled}
      aria-label={ariaLabel}
      className={cn(
        "h-7 px-2 text-sm rounded-lg bg-composer-input text-composer-text border border-composer-border",
        "focus:outline-none focus:border-composer-accent",
        "disabled:cursor-not-allowed disabled:opacity-50",
      )}
    >
      {options.map((opt) => (
        <option key={opt.value} value={opt.value} className="bg-composer-bg-dark">
          {opt.label}
        </option>
      ))}
    </select>
  );
}

// -- Exports ------------------------------------------------------------------

export { Select };
export type { SelectOption };
