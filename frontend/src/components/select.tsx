import { cn } from "@/utils/cn";

// -- Interfaces ----------------------------------------------------------------

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

// -- Components ----------------------------------------------------------------

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
        "rounded-md border border-border bg-surface-elevated px-3 py-2 text-sm text-text",
        "cursor-pointer focus:border-bl-red focus:outline-none focus:ring-1 focus:ring-bl-red",
        "disabled:cursor-not-allowed disabled:opacity-50",
      )}
    >
      {options.map((opt) => (
        <option key={opt.value} value={opt.value}>
          {opt.label}
        </option>
      ))}
    </select>
  );
}

export { Select };
export type { SelectOption };
