import { cn } from "@/utils/cn";

// -- Interfaces ----------------------------------------------------------------

interface NumberInputProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  ariaLabel?: string;
}

// -- Components ----------------------------------------------------------------

const NumberInput: React.FC<NumberInputProps> = ({
  value,
  onChange,
  min,
  max,
  step,
  disabled,
  ariaLabel,
}) => {
  return (
    <input
      type="number"
      value={Number.isFinite(value) ? value : ""}
      min={min}
      max={max}
      step={step}
      disabled={disabled}
      aria-label={ariaLabel}
      onChange={(e) => {
        const next = Number(e.target.value);
        if (Number.isFinite(next)) onChange(next);
      }}
      className={cn(
        "w-32 rounded-md border border-border bg-surface-elevated px-3 py-2 text-sm text-text",
        "focus:border-bl-red focus:outline-none focus:ring-1 focus:ring-bl-red",
        "disabled:cursor-not-allowed disabled:opacity-50",
      )}
    />
  );
};

export { NumberInput };
