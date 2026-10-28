import { CheckCircle, CloudArrowUp } from "@phosphor-icons/react";
import { useRef } from "react";
import { Link } from "react-router";
import { Cover } from "../../components/cover/Cover";
import { ProgressBar, ProgressRing } from "../../components/Progress";
import type { Book } from "../../data/types";
import { formatPercent } from "../../lib/format";
import { markOpening, useReturningCover } from "./coverTransition";
import { DeviceState } from "./DeviceState";
import { isFinished } from "./library";
import styles from "./BookGrid.module.css";

export function BookGrid({ books }: { books: Book[] }) {
  return (
    <ul className={styles.grid}>
      {books.map((book) => (
        <li key={book.id}>
          <GridItem book={book} />
        </li>
      ))}
    </ul>
  );
}

function GridItem({ book }: { book: Book }) {
  const finished = isFinished(book);
  const queued = book.sync.kind === "queued";
  const coverRef = useRef<HTMLDivElement>(null);
  useReturningCover(book.id, coverRef);

  return (
    <Link to={`/read/${book.id}`} viewTransition className={styles.item} onClick={() => markOpening(book.id, coverRef.current)}>
      <div className={styles.coverWrap} ref={coverRef}>
        <Cover title={book.title} author={book.author} format={book.format} edition={book.cover} />
        {queued && (
          <span className={styles.queued}>
            <ProgressRing label="Waiting to upload" size={34} />
            <CloudArrowUp className={styles.queuedIcon} size={14} weight="bold" aria-hidden="true" />
          </span>
        )}
      </div>

      <span className={styles.title}>{book.title}</span>

      <span className={styles.meta}>
        {queued ? (
          <span className={styles.status}>Waiting to upload</span>
        ) : finished ? (
          <span className={`${styles.status} ${styles.finished}`}>
            <CheckCircle size={14} weight="fill" aria-hidden="true" />
            Finished
          </span>
        ) : book.position ? (
          <>
            <ProgressBar value={book.position.progress} label={`${book.title} progress`} className={styles.bar} />
            <span className={`${styles.pct} tabular`}>{formatPercent(book.position.progress)}</span>
          </>
        ) : (
          <span className={styles.status}>Not started</span>
        )}
        {!queued && <DeviceState downloaded={book.downloaded} />}
      </span>
    </Link>
  );
}
