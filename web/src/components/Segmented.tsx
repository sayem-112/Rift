import type { CSSProperties, KeyboardEvent, ReactNode } from "react";
import styles from "./Segmented.module.css";

export interface SegmentedOption<T extends string> {
  value: T;
  label: string;
  icon?: ReactNode;
}

interface SegmentedProps<T extends string> {
  label: string;
  options: SegmentedOption<T>[];
  value: T;
  onChange: (value: T) => void;
  /** Show labels next to icons; icon-only otherwise. */
  showLabels?: boolean;
}

/** A pill radio group with a sliding thumb. Arrow keys move the selection. */
export function Segmented<T extends string>({ label, options, value, onChange, showLabels = true }: SegmentedProps<T>) {
  const index = Math.max(0, options.findIndex((o) => o.value === value));

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const delta = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1 : event.key === "ArrowLeft" || event.key === "ArrowUp" ? -1 : 0;
    if (!delta) return;
    event.preventDefault();
    const next = options[(index + delta + options.length) % options.length];
    if (next) {
      onChange(next.value);
      const buttons = event.currentTarget.querySelectorAll<HTMLButtonElement>("[role=radio]");
      buttons[(index + delta + options.length) % options.length]?.focus();
    }
  };

  return (
    <div
      className={styles.track}
      role="radiogroup"
      aria-label={label}
      onKeyDown={onKeyDown}
      style={{ "--count": options.length, "--index": index } as CSSProperties}
    >
      <span className={styles.thumb} aria-hidden="true" />
      {options.map((option) => {
        const checked = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={checked}
            aria-label={showLabels ? undefined : option.label}
            tabIndex={checked ? 0 : -1}
            className={styles.option}
            onClick={() => onChange(option.value)}
          >
            {option.icon}
            {showLabels && <span>{option.label}</span>}
          </button>
        );
      })}
    </div>
  );
}
