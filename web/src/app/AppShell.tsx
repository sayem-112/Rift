import { useEffect } from "react";
import { Outlet } from "react-router";
import { CommandPalette } from "../components/palette/CommandPalette";
import { openPalette, paletteOpenStore } from "../components/palette/paletteStore";
import { Toaster } from "../components/toast/Toaster";
import { AddBooksProvider, usePickBooks } from "../features/library/AddBooks";
import { useLibrary } from "../features/library/libraryStore";
import { Sidebar } from "./Sidebar";
import styles from "./AppShell.module.css";

/**
 * Desktop: sidebar on the black canvas beside an inset, rounded panel.
 * Phones: the panel goes full-bleed and each page brings its own glass bars.
 */
export function AppShell() {
  return (
    <AddBooksProvider>
      <a href="#main" className={styles.skip}>
        Skip to content
      </a>
      <div className={styles.shell}>
        <Sidebar />
        <div className={styles.panel} id="panel">
          <Outlet />
        </div>
      </div>
      <GlobalPalette />
      <Toaster />
    </AddBooksProvider>
  );
}

function GlobalPalette() {
  const { books } = useLibrary();
  const pickBooks = usePickBooks();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        paletteOpenStore.get() ? paletteOpenStore.set(false) : openPalette();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return <CommandPalette books={books} onAddBook={pickBooks} />;
}
