import "@fontsource-variable/literata";
import { MotionConfig } from "motion/react";
import { useEffect } from "react";
import { Closing } from "./Closing";
import { Hero } from "./Hero";
import { HowItWorks } from "./HowItWorks";
import { LandingNav } from "./LandingNav";
import { Privacy } from "./Privacy";
import { Showcase } from "./Showcase";
import styles from "./Landing.module.css";

/** The public landing page. Loaded on its own, so it never weighs on the app. */
export default function LandingPage() {
  useEffect(() => {
    const previous = document.title;
    document.title = "rift · Your books, on every device";
    return () => {
      document.title = previous;
    };
  }, []);

  return (
    <MotionConfig reducedMotion="user">
      <div className={styles.page}>
        <a href="#top" className={styles.skip}>
          Skip to content
        </a>
        <LandingNav />
        <main id="top">
          <Hero />
          <HowItWorks />
          <Showcase />
          <Privacy />
          <Closing />
        </main>
      </div>
    </MotionConfig>
  );
}
