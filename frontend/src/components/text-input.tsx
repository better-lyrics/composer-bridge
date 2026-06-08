import { cn } from "@/utils/cn";

// -- Interfaces ----------------------------------------------------------------

interface TextInputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  ariaLabel?: string;
  multiline?: boolean;
  rows?: number;
  className?: string;
}

// -- Components ----------------------------------------------------------------

const TextInput: React.FC<TextInputProps> = ({
  value,
  onChange,
  placeholder,
  disabled,
  ariaLabel,
  multiline,
  rows = 3,
  className,
}) => {
  const baseClass = cn(
    "w-full rounded-md border border-border bg-surface-elevated px-3 py-2 text-sm text-text",
    "placeholder:text-text-muted focus:border-bl-red focus:outline-none focus:ring-1 focus:ring-bl-red",
    "disabled:cursor-not-allowed disabled:opacity-50",
    className,
  );
  if (multiline) {
    return (
      <textarea
        rows={rows}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        aria-label={ariaLabel}
        className={cn(baseClass, "font-mono")}
      />
    );
  }
  return (
    <input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      disabled={disabled}
      aria-label={ariaLabel}
      className={baseClass}
    />
  );
};

export { TextInput };
