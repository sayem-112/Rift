import { GearSix, MagnifyingGlass, Plus, type Icon } from "@phosphor-icons/react";
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { useNavigate } from "react-router";
import type { Book } from "../../data/types";
import { fuzzyMatch } from "../../lib/fuzzy";
import { formatPercent } from "../../lib/format";
import { useStore } from "../../lib/store";
import { Cover } from "../cover/Cover";
import { Glass } from "../glass/Glass";
import { closePalette, paletteOpenStore } from "./paletteStore";
import styles from "./CommandPalette.module.css";

interface CommandPaletteProps {
  books: Book[];
  onAddBook: () => void;
}

type Item =
  | { kind: "book"; id: string; book: Book; titleIndices: number[] }
  | { kind: "action"; id: string; label: string; icon: Icon; run: () => void };

const MAX_BOOKS = 8;

/** Ctrl/⌘ K: fuzzy search over titles and authors, plus quick actions. */
export function CommandPalette({ books, onAddBook }: CommandPaletteProps) {
  const open = useStore(paletteOpenStore);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const listId = useId();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      setQuery("");
      setActive(0);
      dialog.showModal();
      inputRef.current?.focus();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);

  const items = useMemo<Item[]>(() => {
    const ranked = books
      .map((book) => {
        const title = fuzzyMatch(query, book.title);
        const author = book.author ? fuzzyMatch(query, book.author) : undefined;
        const best = Math.max(title?.score ?? -Infinity, (author?.score ?? -Infinity) - 0.5);
        return { book, score: best, titleIndices: title?.indices ?? [] };
      })
      .filter((r) => r.score > -Infinity)
      .sort((a, b) => b.score - a.score)
      .slice(0, MAX_BOOKS);

    const actions: Item[] = [
      { kind: "action", id: "add", label: "Add book", icon: Plus, run: onAddBook },
      { kind: "action", id: "settings", label: "Settings", icon: GearSix, run: () => navigate("/settings") },
    ].filter((a) => fuzzyMatch(query, a.label)) as Item[];

    return [...ranked.map((r) => ({ kind: "book" as const, id: r.book.id, book: r.book, titleIndices: r.titleIndices })), ...actions];
  }, [books, query, onAddBook, navigate]);

  const activeIndex = Math.min(active, Math.max(0, items.length - 1));

  const choose = (item: Item | undefined) => {
    if (!item) return;
    closePalette();
    if (item.kind === "book") navigate(`/read/${item.book.id}`, { viewTransition: true });
    else item.run();
  };

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const delta = event.key === "ArrowDown" ? 1 : -1;
      setActive((activeIndex + delta + items.length) % Math.max(1, items.length));
    } else if (event.key === "Enter") {
      event.preventDefault();
      choose(items[activeIndex]);
    }
  };

  useEffect(() => {
    document.getElementById(`${listId}-${activeIndex}`)?.scrollIntoView({ block: "nearest" });
  }, [activeIndex, listId]);

  const bookItems = items.filter((i) => i.kind === "book");
  const actionItems = items.filter((i) => i.kind === "action");

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      aria-label="Search your library"
      onClose={closePalette}
      onClick={(e) => {
        if (e.target === e.currentTarget) closePalette();
      }}
    >
      <Glass radius={24} floating refraction={0} className={styles.panel}>
        <div className={styles.searchRow}>
          <MagnifyingGlass size={20} className={styles.searchIcon} aria-hidden="true" />
          <input
            ref={inputRef}
            className={styles.input}
            placeholder="Search titles and authors"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            onKeyDown={onKeyDown}
            role="combobox"
            aria-expanded="true"
            aria-controls={listId}
            aria-activedescendant={items.length ? `${listId}-${activeIndex}` : undefined}
            aria-autocomplete="list"
            autoComplete="off"
            spellCheck={false}
          />
          <kbd className={styles.kbd}>Esc</kbd>
        </div>

        <div className={styles.results} role="listbox" id={listId} aria-label="Results">
          {items.length === 0 && <p className={styles.empty}>No books match “{query.trim()}”.</p>}
          {bookItems.length > 0 && (
            <Group label="Books">
              {bookItems.map((item) => {
                const index = items.indexOf(item);
                const { book } = item;
                return (
                  <Option key={item.id} id={`${listId}-${index}`} active={index === activeIndex} onHover={() => setActive(index)} onChoose={() => choose(item)}>
                    <span className={styles.thumb}>
                      <Cover title={book.title} author={book.author} format={book.format} edition={book.cover} />
                    </span>
                    <span className={styles.optionText}>
                      <span className={styles.optionTitle}>
                        <Highlight text={book.title} indices={item.titleIndices} />
                      </span>
                      {book.author && <span className={styles.optionMeta}>{book.author}</span>}
                    </span>
                    {book.position && <span className={`${styles.optionMeta} tabular`}>{formatPercent(book.position.progress)}</span>}
                  </Option>
                );
              })}
            </Group>
          )}
          {actionItems.length > 0 && (
            <Group label="Actions">
              {actionItems.map((item) => {
                const index = items.indexOf(item);
                if (item.kind !== "action") return null;
                const ActionIcon = item.icon;
                return (
                  <Option key={item.id} id={`${listId}-${index}`} active={index === activeIndex} onHover={() => setActive(index)} onChoose={() => choose(item)}>
                    <span className={styles.actionIcon}>
                      <ActionIcon size={18} aria-hidden="true" />
                    </span>
                    <span className={styles.optionTitle}>{item.label}</span>
                  </Option>
                );
              })}
            </Group>
          )}
        </div>
      </Glass>
    </dialog>
  );
}

function Group({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div role="group" aria-label={label} className={styles.group}>
      <div className={styles.groupLabel} aria-hidden="true">
        {label}
      </div>
      {children}
    </div>
  );
}

function Option({
  id,
  active,
  onHover,
  onChoose,
  children,
}: {
  id: string;
  active: boolean;
  onHover: () => void;
  onChoose: () => void;
  children: ReactNode;
}) {
  return (
    <div
      id={id}
      role="option"
      aria-selected={active}
      className={styles.option}
      onPointerMove={onHover}
      onClick={onChoose}
    >
      {children}
    </div>
  );
}

function Highlight({ text, indices }: { text: string; indices: number[] }) {
  if (indices.length === 0) return <>{text}</>;
  const marked = new Set(indices);
  return (
    <>
      {[...text].map((ch, i) =>
        marked.has(i) ? (
          <mark key={i} className={styles.mark}>
            {ch}
          </mark>
        ) : (
          <span key={i}>{ch}</span>
        ),
      )}
    </>
  );
}
