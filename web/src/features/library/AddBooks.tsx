import { FilePlus } from "@phosphor-icons/react";
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { Glass } from "../../components/glass/Glass";
import { addFiles } from "./libraryStore";
import styles from "./AddBooks.module.css";

const AddBooksContext = createContext<() => void>(() => {});

/** Opens the system file picker for PDFs and EPUBs. */
export const usePickBooks = () => useContext(AddBooksContext);

/**
 * Owns every way a book enters the library on this device: the file picker
 * and drag-and-drop anywhere on the window. (The Android share target lands
 * here too once the service worker exists.)
 */
export function AddBooksProvider({ children }: { children: ReactNode }) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const pick = useCallback(() => inputRef.current?.click(), []);

  useEffect(() => {
    let depth = 0;
    const hasFiles = (e: DragEvent) => e.dataTransfer?.types.includes("Files") ?? false;

    const onEnter = (e: DragEvent) => {
      if (!hasFiles(e)) return;
      depth++;
      setDragging(true);
    };
    const onLeave = (e: DragEvent) => {
      if (!hasFiles(e)) return;
      depth = Math.max(0, depth - 1);
      if (depth === 0) setDragging(false);
    };
    const onOver = (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      if (e.dataTransfer) e.dataTransfer.dropEffect = "copy";
    };
    const onDrop = (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      depth = 0;
      setDragging(false);
      if (e.dataTransfer?.files.length) addFiles(e.dataTransfer.files);
    };

    window.addEventListener("dragenter", onEnter);
    window.addEventListener("dragleave", onLeave);
    window.addEventListener("dragover", onOver);
    window.addEventListener("drop", onDrop);
    return () => {
      window.removeEventListener("dragenter", onEnter);
      window.removeEventListener("dragleave", onLeave);
      window.removeEventListener("dragover", onOver);
      window.removeEventListener("drop", onDrop);
    };
  }, []);

  return (
    <AddBooksContext.Provider value={pick}>
      {children}
      <input
        ref={inputRef}
        type="file"
        accept=".pdf,.epub,application/pdf,application/epub+zip"
        multiple
        hidden
        onChange={(e) => {
          if (e.target.files?.length) addFiles(e.target.files);
          e.target.value = "";
        }}
      />
      {dragging && (
        <div className={styles.dropzone} aria-hidden="true">
          <Glass radius={28} floating refraction={0} className={styles.dropCard}>
            <FilePlus size={32} weight="duotone" className={styles.dropIcon} />
            <p className={styles.dropTitle}>Drop to add to your library</p>
            <p className={styles.dropHint}>PDF or EPUB, up to 50 MB each</p>
          </Glass>
        </div>
      )}
    </AddBooksContext.Provider>
  );
}
