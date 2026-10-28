import type { CSSProperties } from "react";
import styles from "./Progress.module.css";

interface ProgressBarProps {
  value: number;
  /** Accessible name, e.g. "Meditations progress". */
  label: string;
  tone?: "accent" | "success";
  size?: "sm" | "md";
  className?: string;
}

export function ProgressBar({ value, label, tone = "accent", size = "sm", className }: ProgressBarProps) {
  const pct = Math.round(Math.min(1, Math.max(0, value)) * 100);
  return (
    <div
      className={[styles.bar, styles[size], styles[tone], className].filter(Boolean).join(" ")}
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={pct}
      style={{ "--value": `${pct}%` } as CSSProperties}
    />
  );
}

interface ProgressRingProps {
  /** 0..1, or undefined for an indeterminate (waiting) ring. */
  value?: number;
  size?: number;
  label: string;
  tone?: "accent" | "danger";
}

export function ProgressRing({ value, size = 36, label, tone = "accent" }: ProgressRingProps) {
  const stroke = 3;
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const indeterminate = value === undefined;
  const dash = indeterminate ? c * 0.28 : c * Math.min(1, Math.max(0, value));
  return (
    <svg
      className={[styles.ring, styles[tone], indeterminate && styles.spin].filter(Boolean).join(" ")}
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label={label}
    >
      <circle cx={size / 2} cy={size / 2} r={r} className={styles.ringTrack} strokeWidth={stroke} />
      <circle
        cx={size / 2}
        cy={size / 2}
        r={r}
        className={styles.ringValue}
        strokeWidth={stroke}
        strokeDasharray={`${dash} ${c}`}
        transform={`rotate(-90 ${size / 2} ${size / 2})`}
      />
    </svg>
  );
}
