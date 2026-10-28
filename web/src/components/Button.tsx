import type { ButtonHTMLAttributes, ReactNode, Ref } from "react";
import { Link, type LinkProps } from "react-router";
import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "accent" | "secondary" | "ghost";
export type ButtonSize = "sm" | "md" | "lg";

interface StyleProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Square, icon-only button. Requires an aria-label. */
  iconOnly?: boolean;
  icon?: ReactNode;
}

export function buttonClass({ variant = "secondary", size = "md", iconOnly = false }: StyleProps, extra?: string) {
  return [styles.button, styles[variant], styles[size], iconOnly && styles.iconOnly, extra].filter(Boolean).join(" ");
}

export interface ButtonProps extends StyleProps, ButtonHTMLAttributes<HTMLButtonElement> {
  ref?: Ref<HTMLButtonElement>;
}

export function Button({ variant, size, iconOnly, icon, className, children, type = "button", ...rest }: ButtonProps) {
  return (
    <button type={type} className={buttonClass({ variant, size, iconOnly }, className)} {...rest}>
      {icon}
      {children}
    </button>
  );
}

export interface ButtonLinkProps extends StyleProps, LinkProps {}

export function ButtonLink({ variant, size, iconOnly, icon, className, children, ...rest }: ButtonLinkProps) {
  return (
    <Link className={buttonClass({ variant, size, iconOnly }, className)} {...rest}>
      {icon}
      {children}
    </Link>
  );
}
