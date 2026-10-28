import type { Book } from "../../data/types";

/** Progress at which a book counts as finished without a manual override. */
export const FINISHED_AT = 0.98;

export const BOOK_SIZE_LIMIT = 50 * 1024 * 1024;
export const LIBRARY_QUOTA = 200 * 1024 * 1024;

export type FilterId = "all" | "reading" | "downloaded" | "finished";

export interface FilterDef {
  id: FilterId;
  label: string;
  path: string;
  emptyTitle: string;
  emptyBody: string;
}

export const FILTERS: FilterDef[] = [
  {
    id: "all",
    label: "Library",
    path: "/",
    emptyTitle: "Add your first book",
    emptyBody: "PDFs and EPUBs up to 50 MB. They'll be on every device you sign in to.",
  },
  {
    id: "reading",
    label: "Reading",
    path: "/reading",
    emptyTitle: "Nothing in progress",
    emptyBody: "Books you've started show up here, ready to pick up on any device.",
  },
  {
    id: "downloaded",
    label: "On this device",
    path: "/downloaded",
    emptyTitle: "Nothing saved offline",
    emptyBody: "Books you open here are kept on this device, so they work without a connection.",
  },
  {
    id: "finished",
    label: "Finished",
    path: "/finished",
    emptyTitle: "No finished books yet",
    emptyBody: "Books land here once you reach the end.",
  },
];

export function filterById(id: FilterId): FilterDef {
  return FILTERS.find((f) => f.id === id) ?? FILTERS[0]!;
}

export function isFinished(book: Book): boolean {
  return book.finished ?? (book.position?.progress ?? 0) >= FINISHED_AT;
}

export function isReading(book: Book): boolean {
  return !isFinished(book) && (book.position?.progress ?? 0) > 0;
}

export function matchesFilter(book: Book, filter: FilterId): boolean {
  switch (filter) {
    case "all":
      return true;
    case "reading":
      return isReading(book);
    case "downloaded":
      return book.downloaded;
    case "finished":
      return isFinished(book);
  }
}

/** Most recently read first; never-opened books by date added. */
export function sortByRecent(books: Book[]): Book[] {
  const key = (b: Book) => (b.position?.readAt ?? b.addedAt).getTime();
  return [...books].sort((a, b) => key(b) - key(a));
}

/** The book to hand off: the most recently read unfinished book on any device. */
export function continueReading(books: Book[]): Book | undefined {
  return sortByRecent(books.filter(isReading))[0];
}

export function usedBytes(books: Book[]): number {
  return books.reduce((sum, b) => sum + b.sizeBytes, 0);
}
