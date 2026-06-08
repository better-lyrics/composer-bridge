import { cn } from "@/utils/cn";

// -- Interfaces ---------------------------------------------------------------

interface NumberInputProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  ariaLabel?: string;
}

// -- Component ----------------------------------------------------------------

const NumberInput: React.FC<NumberInputProps> = ({
  value,
  onChange,
  min,
  max,
  step,
  disabled,
  ariaLabel,
}) => (
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
      "h-7 w-20 px-2 text-sm font-mono rounded-md bg-composer-input border border-composer-border text-composer-text",
      "focus:outline-none focus:border-composer-accent",
      "disabled:cursor-not-allowed disabled:opacity-50",
    )}
  />
);

// -- Exports ------------------------------------------------------------------

export { NumberInput };
