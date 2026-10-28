import { Play } from "@phosphor-icons/react";
import { useRef, type CSSProperties } from "react";
import { ButtonLink } from "../../components/Button";
import { Cover } from "../../components/cover/Cover";
import { coverGlow } from "../../components/cover/coverArt";
import { Glass } from "../../components/glass/Glass";
import { ProgressBar } from "../../components/Progress";
import type { Book, Device } from "../../data/types";
import { formatPercent, formatRelative } from "../../lib/format";
import { markOpening, useReturningCover } from "./coverTransition";
import { DEVICE_ICONS } from "./deviceIcon";
import styles from "./ContinueCard.module.css";

/**
 * The handoff: the book last read on any device. A liquid-glass slab floats
 * over a wash of the book's own cover light, and says where and when it was
 * left, with one action to continue.
 */
export function ContinueCard({ book, devices }: { book: Book; devices: Device[] }) {
  const coverRef = useRef<HTMLDivElement>(null);
  useReturningCover(book.id, coverRef);

  const position = book.position;
  if (!position) return null;

  const device = devices.find((d) => d.id === position.deviceId);
  const DeviceIcon = DEVICE_ICONS[device?.kind ?? "phone"];
  const where = device?.isCurrent ? "this device" : (device?.name ?? "another device");
  const art = coverGlow(book.title, book.cover);
  const open = () => markOpening(book.id, coverRef.current);

  return (
    <section
      className={styles.card}
      aria-labelledby={`continue-${book.id}`}
      style={{ "--glow-a": art.from, "--glow-b": art.to, "--glow-angle": `${art.angle}deg` } as CSSProperties}
    >
      <h2 className="visually-hidden">Continue reading</h2>
      <div className={styles.light} aria-hidden="true" />

      <Glass radius={22} refraction={48} tint="var(--ctp-crust)" className={styles.pane}>
        <div className={styles.cover} ref={coverRef}>
          <Cover title={book.title} author={book.author} format={book.format} edition={book.cover} />
        </div>

        <div className={styles.body}>
          <h3 className={styles.title} id={`continue-${book.id}`}>
            {book.title}
          </h3>
          <p className={`${styles.meta} tabular`}>
            <span className={styles.pct}>{formatPercent(position.progress)}</span>
            <span aria-hidden="true">·</span>
            <span>{position.label}</span>
            <span aria-hidden="true" className={styles.deviceSep}>
              ·
            </span>
            <span className={styles.device}>
              <DeviceIcon size={14} weight="fill" aria-hidden="true" />
              from {where}, {formatRelative(position.readAt)}
            </span>
          </p>
          <ProgressBar value={position.progress} label={`${book.title} progress`} size="md" className={styles.bar} />
        </div>

        <ButtonLink
          to={`/read/${book.id}`}
          viewTransition
          onClick={open}
          variant="accent"
          size="lg"
          icon={<Play size={16} weight="fill" aria-hidden="true" />}
          className={styles.resume}
        >
          Resume reading
        </ButtonLink>
      </Glass>
    </section>
  );
}
