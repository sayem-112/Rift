import { useMemo } from "react";
import { SAMPLE_BOOKS, SAMPLE_DEVICES } from "../../data/sampleLibrary";
import type { Book, Device } from "../../data/types";
import { titleFromFilename } from "../../lib/format";
import { sampleLibraryStore } from "../../lib/prefs";
import { createStore, useStore } from "../../lib/store";
import { toast } from "../../components/toast/toastStore";
import { usedBytes } from "./library";
import { checkFile } from "./validateFile";

/**
 * Client-side library state. Until the sync API exists this holds the sample
 * library (when enabled) plus books added on this device, which wait in a
 * "queued" sync state.
 */

const localBooksStore = createStore<Book[]>([]);

export function useLibrary(): { books: Book[]; devices: Device[]; isSample: boolean } {
  const showSamples = useStore(sampleLibraryStore);
  const local = useStore(localBooksStore);
  return useMemo(
    () => ({
      books: showSamples ? [...local, ...SAMPLE_BOOKS] : local,
      devices: showSamples ? SAMPLE_DEVICES : SAMPLE_DEVICES.filter((d) => d.isCurrent),
      isSample: showSamples,
    }),
    [showSamples, local],
  );
}

export function deviceName(devices: Device[], id: string): string {
  const device = devices.find((d) => d.id === id);
  if (!device) return "another device";
  return device.isCurrent ? "this device" : device.name;
}

/** Validates and adds files from the picker, a drop, or the share target. */
export function addFiles(files: Iterable<File>): void {
  const current = [...localBooksStore.get(), ...(sampleLibraryStore.get() ? SAMPLE_BOOKS : [])];
  let used = usedBytes(current);
  const added: Book[] = [];

  for (const file of files) {
    const check = checkFile(file, used);
    if (!check.ok) {
      toast({
        tone: "danger",
        message: check.message,
        action: check.reason === "quota" ? { label: "Manage storage", href: "/settings#storage" } : undefined,
      });
      continue;
    }
    used += file.size;
    added.push({
      id: `local-${crypto.randomUUID()}`,
      title: titleFromFilename(file.name),
      format: check.format,
      sizeBytes: file.size,
      addedAt: new Date(),
      downloaded: true,
      sync: { kind: "queued" },
    });
  }

  if (added.length === 0) return;
  localBooksStore.set((prev) => [...added, ...prev]);
  toast({
    tone: "neutral",
    message:
      added.length === 1
        ? `Added “${added[0]!.title}”. It'll upload once sync is connected.`
        : `Added ${added.length} books. They'll upload once sync is connected.`,
  });
}

export function removeLocalBook(id: string): void {
  localBooksStore.set((prev) => prev.filter((b) => b.id !== id));
}
