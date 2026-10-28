import { Plus } from "@phosphor-icons/react";
import { Button } from "../../components/Button";
import { RiftMark } from "../../components/RiftMark";
import styles from "./LibraryEmpty.module.css";

/** First run: one action, and a hint for the other ways in. */
export function LibraryEmpty({ onAdd }: { onAdd: () => void }) {
  return (
    <div className={styles.empty}>
      <RiftMark size={56} glow />
      <h2 className={styles.title}>Add your first book</h2>
      <p className={styles.body}>PDFs and EPUBs up to 50 MB. Add one here and it's on every device you sign in to, open at the page you left.</p>
      <Button variant="primary" size="lg" icon={<Plus size={18} weight="bold" aria-hidden="true" />} onClick={onAdd}>
        Add book
      </Button>
      {/* The phone hint ("or share a file to rift") joins once the share target ships with the service worker. */}
      <p className={styles.hint}>or drop a PDF or EPUB anywhere on this window</p>
    </div>
  );
}
