import { Info, ListBullets, MagnifyingGlass, Plus, SquaresFour } from "@phosphor-icons/react";
import { useMemo } from "react";
import { Button } from "../../components/Button";
import { Glass } from "../../components/glass/Glass";
import { openPalette } from "../../components/palette/paletteStore";
import { Segmented } from "../../components/Segmented";
import { viewStore, type LibraryView } from "../../lib/prefs";
import { useStore } from "../../lib/store";
import { withViewTransition } from "../../lib/viewTransition";
import { usePickBooks } from "./AddBooks";
import { BookGrid } from "./BookGrid";
import { BookList } from "./BookList";
import { ContinueCard } from "./ContinueCard";
import { continueReading, filterById, matchesFilter, sortByRecent, type FilterId } from "./library";
import { useLibrary } from "./libraryStore";
import { LibraryEmpty } from "./LibraryEmpty";
import { MobileHeader } from "./MobileHeader";
import styles from "./LibraryPage.module.css";

const VIEW_OPTIONS = [
  { value: "grid" as const, label: "Grid", icon: <SquaresFour size={16} aria-hidden="true" /> },
  { value: "list" as const, label: "List", icon: <ListBullets size={16} aria-hidden="true" /> },
];

export function LibraryPage({ filter }: { filter: FilterId }) {
  const { books, devices, isSample } = useLibrary();
  const view = useStore(viewStore);
  const pickBooks = usePickBooks();
  const def = filterById(filter);

  const handoff = filter === "all" ? continueReading(books) : undefined;
  const visible = useMemo(
    () => sortByRecent(books.filter((b) => matchesFilter(b, filter) && b.id !== handoff?.id)),
    [books, filter, handoff],
  );
  const total = books.filter((b) => matchesFilter(b, filter)).length;

  const setView = (next: LibraryView) => withViewTransition(() => viewStore.set(next));

  const libraryEmpty = books.length === 0;
  const filterEmpty = !libraryEmpty && total === 0;

  return (
    <>
      <MobileHeader />

      <Glass as="header" radius={18} refraction={52} className={styles.toolbar}>
        <h1 className={styles.title}>
          {def.label}
          <span className={`${styles.titleCount} tabular`}>{total}</span>
        </h1>
        {isSample && (
          <span className={styles.sample} title="These are placeholder books until sync is connected.">
            <Info size={14} weight="bold" aria-hidden="true" />
            Sample library
          </span>
        )}
        <div className={styles.toolbarEnd}>
          {!libraryEmpty && <Segmented label="Layout" options={VIEW_OPTIONS} value={view} onChange={setView} />}
          <Button variant="primary" icon={<Plus size={17} weight="bold" aria-hidden="true" />} onClick={pickBooks}>
            Add book
          </Button>
        </div>
      </Glass>

      <main id="main" className={styles.content} tabIndex={-1}>
        <h1 className="visually-hidden">{def.label}</h1>

        {libraryEmpty ? (
          <LibraryEmpty onAdd={pickBooks} />
        ) : (
          <>
            {handoff && <ContinueCard book={handoff} devices={devices} />}

            {filterEmpty ? (
              <div className={styles.filterEmpty}>
                <h2 className={styles.filterEmptyTitle}>{def.emptyTitle}</h2>
                <p className={styles.filterEmptyBody}>{def.emptyBody}</p>
              </div>
            ) : (
              <section aria-label={handoff ? "More books" : def.label} className={styles.collection}>
                <div className={styles.mobileViewRow}>
                  <span className={`${styles.mobileCount} tabular`}>
                    {total} {total === 1 ? "book" : "books"}
                    {isSample && (
                      <span className={styles.sample}>
                        <Info size={13} weight="bold" aria-hidden="true" />
                        Sample library
                      </span>
                    )}
                  </span>
                  <Segmented label="Layout" options={VIEW_OPTIONS} value={view} onChange={setView} showLabels={false} />
                </div>
                {view === "grid" ? <BookGrid books={visible} /> : <BookList books={visible} devices={devices} />}
              </section>
            )}
          </>
        )}
      </main>

      {!libraryEmpty && (
        <Glass as="nav" radius="pill" floating refraction={56} className={styles.bottomBar} aria-label="Quick actions">
          <Button variant="ghost" iconOnly aria-label="Search" icon={<MagnifyingGlass size={20} aria-hidden="true" />} onClick={openPalette} />
          <Button variant="primary" size="md" icon={<Plus size={18} weight="bold" aria-hidden="true" />} onClick={pickBooks} className={styles.bottomAdd}>
            Add book
          </Button>
        </Glass>
      )}
    </>
  );
}
