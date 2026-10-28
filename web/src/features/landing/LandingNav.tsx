import { ArrowRight } from "@phosphor-icons/react";
import { useMotionValueEvent, useScroll } from "motion/react";
import { useState } from "react";
import { ButtonLink } from "../../components/Button";
import { Glass } from "../../components/glass/Glass";
import { Wordmark } from "../../components/RiftMark";
import styles from "./LandingNav.module.css";

/** Floating glass pill. It tightens once the page scrolls under it. */
export function LandingNav() {
  const { scrollY } = useScroll();
  const [scrolled, setScrolled] = useState(false);
  useMotionValueEvent(scrollY, "change", (y) => setScrolled(y > 24));

  return (
    <div className={styles.wrap}>
      <Glass as="header" radius="pill" floating={scrolled} refraction={0} className={styles.nav} data-scrolled={scrolled || undefined}>
        <a href="#top" className={styles.brand} aria-label="rift, back to top">
          <Wordmark size={20} playful />
        </a>
        <nav aria-label="Sections" className={styles.links}>
          <a href="#how">How it works</a>
          <a href="#library">Library</a>
          <a href="#privacy">Privacy</a>
        </nav>
        <span className={styles.badge}>In development</span>
        <ButtonLink to="/signin" variant="primary" size="sm" className={styles.cta}>
          Open rift
          <ArrowRight size={14} weight="bold" aria-hidden="true" />
        </ButtonLink>
      </Glass>
    </div>
  );
}
