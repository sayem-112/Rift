import type { BookFormat } from "../../data/types";
import { formatBytes } from "../../lib/format";
import { BOOK_SIZE_LIMIT, LIBRARY_QUOTA } from "./library";

export type FileCheck =
  | { ok: true; format: BookFormat }
  | { ok: false; reason: "type" | "size" | "quota"; message: string };

export function detectFormat(file: Pick<File, "name" | "type">): BookFormat | undefined {
  const name = file.name.toLowerCase();
  if (file.type === "application/pdf" || name.endsWith(".pdf")) return "pdf";
  if (file.type === "application/epub+zip" || name.endsWith(".epub")) return "epub";
  return undefined;
}

/** Checks one file against the format, per-book and per-library limits. */
export function checkFile(file: Pick<File, "name" | "type" | "size">, usedBytes: number): FileCheck {
  const format = detectFormat(file);
  if (!format) {
    return { ok: false, reason: "type", message: `“${file.name}” isn't a PDF or EPUB.` };
  }
  if (file.size > BOOK_SIZE_LIMIT) {
    return {
      ok: false,
      reason: "size",
      message: `“${file.name}” is ${formatBytes(file.size)}. Books can be up to ${formatBytes(BOOK_SIZE_LIMIT)}.`,
    };
  }
  if (usedBytes + file.size > LIBRARY_QUOTA) {
    return {
      ok: false,
      reason: "quota",
      message: `No room for “${file.name}”. ${formatBytes(usedBytes)} of ${formatBytes(LIBRARY_QUOTA)} used.`,
    };
  }
  return { ok: true, format };
}
