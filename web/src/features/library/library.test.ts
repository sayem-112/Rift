import { describe, expect, it } from "vitest";
import type { Book } from "../../data/types";
import { continueReading, isFinished, matchesFilter } from "./library";
import { checkFile } from "./validateFile";

const MB = 1024 * 1024;

function book(overrides: Partial<Book>): Book {
  return {
    id: "b",
    title: "Book",
    format: "epub",
    sizeBytes: MB,
    addedAt: new Date(0),
    downloaded: false,
    sync: { kind: "synced" },
    ...overrides,
  };
}

const at = (progress: number, readAt: number) => ({ progress, label: "", deviceId: "d", readAt: new Date(readAt) });

describe("filters", () => {
  it("treats 98% as finished unless overridden", () => {
    expect(isFinished(book({ position: at(0.98, 1) }))).toBe(true);
    expect(isFinished(book({ position: at(0.99, 1), finished: false }))).toBe(false);
    expect(isFinished(book({ finished: true }))).toBe(true);
  });

  it("only counts started, unfinished books as reading", () => {
    expect(matchesFilter(book({}), "reading")).toBe(false);
    expect(matchesFilter(book({ position: at(0.4, 1) }), "reading")).toBe(true);
    expect(matchesFilter(book({ position: at(1, 1) }), "reading")).toBe(false);
  });

  it("picks the most recently read unfinished book to continue", () => {
    const books = [
      book({ id: "old", position: at(0.3, 100) }),
      book({ id: "done", position: at(1, 500) }),
      book({ id: "new", position: at(0.5, 300) }),
    ];
    expect(continueReading(books)?.id).toBe("new");
  });
});

describe("checkFile", () => {
  it("accepts PDFs and EPUBs by extension or type", () => {
    expect(checkFile({ name: "a.PDF", type: "", size: MB }, 0)).toEqual({ ok: true, format: "pdf" });
    expect(checkFile({ name: "a", type: "application/epub+zip", size: MB }, 0)).toEqual({ ok: true, format: "epub" });
  });

  it("rejects other types, oversized books and full libraries", () => {
    expect(checkFile({ name: "a.mobi", type: "", size: MB }, 0)).toMatchObject({ ok: false, reason: "type" });
    expect(checkFile({ name: "a.pdf", type: "", size: 87 * MB }, 0)).toMatchObject({
      ok: false,
      reason: "size",
      message: "“a.pdf” is 87 MB. Books can be up to 50 MB.",
    });
    expect(checkFile({ name: "a.pdf", type: "", size: 20 * MB }, 190 * MB)).toMatchObject({ ok: false, reason: "quota" });
  });
});
