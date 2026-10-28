import styles from "./RiftMark.module.css";

interface RiftMarkProps {
  size?: number;
  /** Soft bloom behind the mark, for the sign-in and empty screens. */
  glow?: boolean;
  /**
   * The two halves drift apart on hover and click back together, for marks
   * people can point at (the landing page, the brand link).
   */
  playful?: boolean;
  className?: string;
}

/*
 * The mark: a bookmark torn across, its halves knocked slightly apart.
 * A bookmark is your place in a book; the tear is the rift it crosses
 * between devices. The upper half is the accent, the lower a paler tint
 * of it, and the gap between them is the page showing through.
 */
const TOP = "M3 4.6 Q3 1 6.6 1 H17.4 Q21 1 21 4.6 V10.4 L13.6 14.9 L9.6 11.4 L3 15.4 Z";
const BOTTOM = "M3 15.4 L9.6 11.4 L13.6 14.9 L21 10.4 V29 L12 22.6 L3 29 Z";

export function RiftMark({ size = 20, glow = false, playful = false, className }: RiftMarkProps) {
  return (
    <span
      className={[styles.mark, glow && styles.glow, playful && styles.playful, className].filter(Boolean).join(" ")}
      style={{ height: size }}
      aria-hidden="true"
    >
      <svg viewBox="0 -1 24 32" height={size} width={(size * 24) / 32}>
        <path className={styles.top} d={TOP} />
        <path className={styles.bottom} d={BOTTOM} />
      </svg>
    </span>
  );
}

export function Wordmark({ size = 20, playful = false }: { size?: number; playful?: boolean }) {
  return (
    <span className={[styles.wordmark, playful && styles.wordmarkPlayful].filter(Boolean).join(" ")} style={{ fontSize: size }}>
      <RiftMark size={size * 1.05} playful={playful} />
      <span>rift</span>
    </span>
  );
}
