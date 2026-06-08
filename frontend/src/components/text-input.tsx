import { cn } from "@/utils/cn";

// -- Interfaces ---------------------------------------------------------------

interface TextInputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  ariaLabel?: string;
  multiline?: boolean;
  rows?: number;
  mono?: boolean;
  className?: string;
}

// -- Component ----------------------------------------------------------------

const TextInput: React.FC<TextInputProps> = ({
  value,
  onChange,
  placeholder,
  disabled,
  ariaLabel,
  multiline,
  rows = 3,
  mono,
  className,
}) => {
  const base = cn(
    "w-full rounded-md bg-composer-input border border-composer-border text-composer-text",
    "placeholder:text-composer-text-muted focus:outline-none focus:border-composer-accent",
    "disabled:cursor-not-allowed disabled:opacity-50",
    mono && "font-mono text-xs",
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
        className={cn(base, "px-3 py-2 text-sm resize-none")}
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
      className={cn(base, "h-7 px-2 text-sm")}
    />
  );
};

// -- Exports ------------------------------------------------------------------

export { TextInput };
