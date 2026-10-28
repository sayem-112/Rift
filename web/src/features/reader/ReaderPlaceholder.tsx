import { ArrowLeft } from "@phosphor-icons/react";
import { Link, useParams } from "react-router";
import { ButtonLink } from "../../components/Button";
import { Cover } from "../../components/cover/Cover";
import { coverGlow } from "../../components/cover/coverArt";
import { Glass } from "../../components/glass/Glass";
import { COVER_TRANSITION_NAME } from "../library/coverTransition";
import { useLibrary } from "../library/libraryStore";
import styles from "./ReaderPlaceholder.module.css";

/**
 * Stand-in for the reader (build step 4). It exists so the cover-to-reader
 * shared-element transition and the reader's glass top bar can be judged now.
 */
export function ReaderPlaceholder() {
  const { bookId } = useParams();
  const { books } = useLibrary();
  const book = books.find((b) => b.id === bookId);

  if (!book) {
    return (
      <main className={styles.page}>
        <div className={styles.center}>
          <h1 className={styles.title}>This book isn't in your library</h1>
          <ButtonLink to="/" variant="secondary">
            Back to library
          </ButtonLink>
        </div>
      </main>
    );
  }

  const art = coverGlow(book.title, book.cover);

  return (
    <main className={styles.page}>
      <Glass as="header" radius="pill" floating tint={art.from} className={styles.bar}>
        <Link to="/" viewTransition className={styles.back} aria-label="Back to library">
          <ArrowLeft size={20} aria-hidden="true" />
        </Link>
        <span className={styles.barTitle}>{book.title}</span>
      </Glass>

      <div className={styles.center}>
        <div className={styles.cover} style={{ viewTransitionName: COVER_TRANSITION_NAME }}>
          <Cover title={book.title} author={book.author} format={book.format} edition={book.cover} />
        </div>
        <h1 className={styles.title}>{book.title}</h1>
        <p className={styles.note}>The reader arrives in a later build. {book.position ? `You're at ${book.position.label}.` : ""}</p>
      </div>
    </main>
  );
}
