import { motion, useScroll, useTransform } from "motion/react";
import { useRef } from "react";
import styles from "./Showcase.module.css";
import layout from "./Landing.module.css";

/*
 * Real screenshots of the app (sample library), captured from the running
 * build at 2x. Regenerate them whenever the library UI changes.
 */
export function Showcase() {
  const ref = useRef<HTMLElement>(null);
  const { scrollYProgress } = useScroll({
    target: ref,
    offset: ["start end", "end start"],
  });
  // The phone drifts against the desktop as the section passes: depth without a gimmick.
  const phoneY = useTransform(scrollYProgress, [0, 1], ["10%", "-10%"]);
  const desktopY = useTransform(scrollYProgress, [0, 1], ["4%", "-4%"]);

  return (
    <section ref={ref} id="library" className={`${layout.section} ${styles.showcase}`} aria-labelledby="library-title">
      <div className={layout.container}>
        <div>
          <h2 id="library-title" className={layout.heading}>
            One tap back into your book.
          </h2>
          <p className={layout.lede}>
            The book you're reading sits at the top, with how far you got and which device you used. Below it, every book you've added, as
            covers or as a list.
          </p>
        </div>

        <div className={styles.shots}>
          <motion.figure className={styles.desktop} style={{ y: desktopY }}>
            <img
              src="/landing/library-desktop.webp"
              width={1440}
              height={900}
              alt="rift on a desktop: a sidebar of filters, a Continue reading card for Meditations at 62%, and a grid of book covers."
              loading="lazy"
              decoding="async"
            />
          </motion.figure>
          <motion.figure className={styles.phone} style={{ y: phoneY }}>
            <img
              src="/landing/library-mobile.webp"
              width={390}
              height={844}
              alt="rift on a phone: a glass header with filters, the Continue reading card, and covers in three columns."
              loading="lazy"
              decoding="async"
            />
          </motion.figure>
        </div>
        <p className={styles.note}>Screenshots of the current build, shown with a sample library.</p>
      </div>
    </section>
  );
}
