import { CaretRight, CheckCircle } from "@phosphor-icons/react";
import { useRef } from "react";
import { Link } from "react-router";
import { Cover } from "../../components/cover/Cover";
import { ProgressBar, ProgressRing } from "../../components/Progress";
import type { Book, Device } from "../../data/types";
import { formatPercent, formatRelative } from "../../lib/format";
import { markOpening, useReturningCover } from "./coverTransition";
import { DeviceState } from "./DeviceState";
import { isFinished } from "./library";
import { deviceName } from "./libraryStore";
import styles from "./BookList.module.css";

export function BookList({ books, devices }: { books: Book[]; devices: Device[] }) {
  return (
    <ul className={styles.list}>
      {books.map((book) => (
        <li key={book.id}>
          <ListRow book={book} devices={devices} />
        </li>
      ))}
    </ul>
  );
}

function ListRow({ book, devices }: { book: Book; devices: Device[] }) {
  const finished = isFinished(book);
  const queued = book.sync.kind === "queued";
  const position = book.position;
  const coverRef = useRef<HTMLSpanElement>(null);
  useReturningCover(book.id, coverRef);

  return (
    <Link to={`/read/${book.id}`} viewTransition className={styles.row} onClick={() => markOpening(book.id, coverRef.current)}>
      <span className={styles.cover} ref={coverRef}>
        <Cover title={book.title} author={book.author} format={book.format} edition={book.cover} />
      </span>

      <span className={styles.main}>
        <span className={styles.title}>{book.title}</span>
        <span className={styles.author}>{book.author ?? (book.format === "pdf" ? "PDF" : "EPUB")}</span>
      </span>

      <span className={styles.progress}>
        {queued ? (
          <span className={styles.status}>
            <ProgressRing label="Waiting to upload" size={16} />
            Waiting to upload
          </span>
        ) : finished ? (
          <span className={`${styles.status} ${styles.finished}`}>
            <CheckCircle size={15} weight="fill" aria-hidden="true" />
            Finished
          </span>
        ) : position ? (
          <>
            <ProgressBar value={position.progress} label={`${book.title} progress`} className={styles.bar} />
            <span className={`${styles.pct} tabular`}>{formatPercent(position.progress)}</span>
          </>
        ) : (
          <span className={styles.status}>Not started</span>
        )}
      </span>

      <span className={styles.when}>
        {position ? `${capitalize(deviceName(devices, position.deviceId))} · ${formatRelative(position.readAt)}` : `Added ${formatRelative(book.addedAt)}`}
      </span>

      <span className={styles.end}>
        {!queued && <DeviceState downloaded={book.downloaded} />}
        <CaretRight size={15} aria-hidden="true" className={styles.caret} />
      </span>
    </Link>
  );
}

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}
