import type { CSSProperties } from "react";
import type { BookFormat } from "../../data/types";
import { coverArt } from "./coverArt";
import { EDITIONS } from "./editions";
import styles from "./Cover.module.css";

interface CoverProps {
  title: string;
  author?: string;
  format: BookFormat;
  /** The book's own cover art, when it has one; otherwise a cover is generated from the title. */
  edition?: string;
  className?: string;
}

/**
 * A typographic cover generated from the book's title. Text scales with the
 * cover's own width (container query units), so one component serves 32px
 * thumbnails and 120px grid covers alike.
 */
export function Cover({ title, author, format, edition, className }: CoverProps) {
  const art = edition ? EDITIONS[edition] : undefined;
  if (art) {
    return (
      <div className={[styles.cover, styles.edition, className].filter(Boolean).join(" ")} aria-hidden="true">
        <svg className={styles.art} viewBox="0 0 200 300" preserveAspectRatio="xMidYMid slice">
          {art.art}
        </svg>
      </div>
    );
  }
  return <GeneratedCover title={title} author={author} format={format} className={className} />;
}

function GeneratedCover({ title, author, format, className }: Omit<CoverProps, "edition">) {
  const art = coverArt(title);
  const longestWord = Math.max(...title.split(/\s+/).map((w) => w.length));
  // Step down only when a word would not fit the cover's width, or the title would run past five lines.
  const length = longestWord > 12 || title.length > 44 ? "longest" : longestWord > 10 || title.length > 30 ? "long" : undefined;
  const style = {
    "--cover-from": art.from,
    "--cover-to": art.to,
    "--cover-angle": `${art.angle}deg`,
    "--cover-glint": `${art.glint}%`,
  } as CSSProperties;

  return (
    <div className={[styles.cover, className].filter(Boolean).join(" ")} style={style} data-length={length} aria-hidden="true">
      <div className={styles.inner}>
        <span className={styles.format}>{format === "pdf" ? "PDF" : "EPUB"}</span>
        <span className={styles.text}>
          {author && <span className={styles.author}>{author}</span>}
          <span className={styles.title}>{title}</span>
        </span>
      </div>
    </div>
  );
}
