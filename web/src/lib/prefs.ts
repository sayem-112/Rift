import { DEFAULT_ACCENT, accentHex, isAccentId, type AccentId } from "./catppuccin";
import { createPersistedStore } from "./store";

/** Per-device display preferences. These never sync, by design. */

export type LibraryView = "grid" | "list";

export const accentStore = createPersistedStore<AccentId>("rift.accent", DEFAULT_ACCENT, isAccentId);

export const viewStore = createPersistedStore<LibraryView>(
  "rift.libraryView",
  "grid",
  (v): v is LibraryView => v === "grid" || v === "list",
);

/** Development only: show the sample library until the sync API exists. */
export const sampleLibraryStore = createPersistedStore<boolean>(
  "rift.sampleLibrary",
  true,
  (v): v is boolean => typeof v === "boolean",
);

function applyAccent(id: AccentId) {
  document.documentElement.style.setProperty("--accent", accentHex(id));
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", "#000000");
}

applyAccent(accentStore.get());
accentStore.subscribe(() => applyAccent(accentStore.get()));
