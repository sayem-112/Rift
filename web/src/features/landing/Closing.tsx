import { ArrowRight } from "@phosphor-icons/react";
import { useInView } from "motion/react";
import { useRef } from "react";
import { ButtonLink } from "../../components/Button";
import { RiftMark, Wordmark } from "../../components/RiftMark";
import styles from "./Closing.module.css";
import layout from "./Landing.module.css";

export function Closing() {
  const ref = useRef<HTMLDivElement>(null);
  // The mark arrives torn wide open and snaps shut: the last handoff on the page.
  const inView = useInView(ref, { once: true, amount: 0.6 });

  return (
    <>
      <section className={`${layout.section} ${styles.closing}`} aria-labelledby="closing-title">
        <div ref={ref} className={styles.inner}>
          <RiftMark size={80} glow playful className={inView ? undefined : styles.apart} />
          <h2 id="closing-title" className={styles.title}>
            Your next page is already waiting.
          </h2>
          <ButtonLink to="/signin" variant="primary" size="lg" className={styles.cta}>
            Open rift
            <ArrowRight size={17} weight="bold" aria-hidden="true" />
          </ButtonLink>
          <p className={styles.fine}>PDF and EPUB, up to 50 MB per book. rift is in early development.</p>
        </div>
      </section>

      <footer className={`${layout.container} ${styles.footer}`}>
        <Wordmark size={18} />
        <p>
          Colors from{" "}
          <a href="https://catppuccin.com" target="_blank" rel="noreferrer">
            Catppuccin
          </a>
          . Sample books are public-domain works.
        </p>
      </footer>
    </>
  );
}
