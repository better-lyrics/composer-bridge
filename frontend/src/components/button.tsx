import { cn } from "@/utils/cn";

// -- Interfaces ---------------------------------------------------------------

type ButtonVariant = "primary" | "secondary" | "ghost" | "destructive";
type ButtonSize = "sm" | "md" | "icon";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  hasIcon?: boolean;
}

// -- Constants ----------------------------------------------------------------

const BASE =
  "inline-flex items-center justify-center gap-1.5 font-medium rounded-lg transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed";

const VARIANTS: Record<ButtonVariant, string> = {
  primary: "bg-composer-accent-dark hover:bg-composer-accent text-white",
  secondary: "bg-composer-button hover:bg-composer-button-hover text-composer-text",
  ghost: "text-composer-text-muted hover:text-composer-text hover:bg-composer-button",
  destructive: "bg-composer-error/80 hover:bg-composer-error text-composer-error-text",
};

const SIZES: Record<ButtonSize, { base: string; withIcon: string }> = {
  sm: { base: "h-7 px-2.5 text-xs", withIcon: "h-7 pl-2 pr-3 text-xs" },
  md: { base: "h-8 px-3 text-sm", withIcon: "h-8 pl-2.5 pr-3.5 text-sm" },
  icon: { base: "size-8 p-0 text-sm", withIcon: "size-8 p-0 text-sm" },
};

// -- Component ----------------------------------------------------------------

const Button: React.FC<ButtonProps> = ({
  variant = "secondary",
  size = "md",
  hasIcon,
  className,
  children,
  type = "button",
  ...props
}) => {
  const sizeClasses = hasIcon ? SIZES[size].withIcon : SIZES[size].base;
  return (
    <button type={type} className={cn(BASE, VARIANTS[variant], sizeClasses, className)} {...props}>
      {children}
    </button>
  );
};

// -- Exports ------------------------------------------------------------------

export { Button };
export type { ButtonProps, ButtonVariant, ButtonSize };
