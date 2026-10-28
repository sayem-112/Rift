import { ArrowDown, ArrowRight } from "@phosphor-icons/react";
import { motion } from "motion/react";
import { ButtonLink } from "../../components/Button";
import { HandoffDemo } from "./HandoffDemo";
import styles from "./Hero.module.css";
import layout from "./Landing.module.css";

/*
 * The copy is on screen from the first frame; only the stage settles in,
 * rising a little as the page loads.
 */
export function Hero() {
  return (
    <section className={`${layout.container} ${styles.hero}`} aria-labelledby="hero-title">
      <div className={styles.copy}>
        <h1 id="hero-title" className={styles.title}>
          <span className={styles.titleLine}>Your books. Every device.</span>
          <span className={`${styles.titleLine} ${styles.titleSecond}`}>Exactly where you left off.</span>
        </h1>
        <p className={styles.sub}>
          rift keeps the PDFs and EPUBs you own in sync across your phone, tablet and computer, down to the page. No re-downloading, no
          hunting for your place.
        </p>
        <div className={styles.actions}>
          <ButtonLink to="/signin" variant="primary" size="lg" className={styles.primary}>
            Open rift
            <ArrowRight size={17} weight="bold" aria-hidden="true" />
          </ButtonLink>
          <a href="#how" className={styles.secondary}>
            See how it works
            <ArrowDown size={15} weight="bold" aria-hidden="true" />
          </a>
        </div>
      </div>

      <motion.div
        className={styles.demo}
        initial={{ y: 48 }}
        animate={{ y: 0 }}
        transition={{ type: "spring", stiffness: 70, damping: 18, delay: 0.1 }}
      >
        <HandoffDemo />
      </motion.div>
    </section>
  );
}
