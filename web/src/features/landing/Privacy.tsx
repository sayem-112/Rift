import { Books, Key, LockSimple } from "@phosphor-icons/react";
import { motion } from "motion/react";
import styles from "./Privacy.module.css";
import layout from "./Landing.module.css";

// Wording follows PRODUCT.md: never claim "we can't see them".
const FACTS = [
  {
    icon: LockSimple,
    title: "Encrypted in storage",
    body: "Every book is stored encrypted with its own key. The files can't be read from storage alone.",
  },
  {
    icon: Books,
    title: "Yours, and only yours",
    body: "A personal library of books you own. No store, no sharing between accounts, nothing public.",
  },
  {
    icon: Key,
    title: "No passwords",
    body: "Sign in with Google or a one-time email link. Sign out any device from Settings, instantly.",
  },
];

export function Privacy() {
  return (
    <section id="privacy" className={`${layout.section} ${layout.container}`} aria-labelledby="privacy-title">
      <div className={styles.grid}>
        <div>
          <h2 id="privacy-title" className={layout.heading}>
            Your books stay yours.
          </h2>
          <p className={layout.lede}>Your books are encrypted and private to your account. We never access or share them.</p>
        </div>

        <ul className={styles.facts}>
          {FACTS.map(({ icon: Icon, title, body }, i) => (
            <motion.li
              key={title}
              className={styles.fact}
              initial={{ y: 14 }}
              whileInView={{ y: 0 }}
              viewport={{ once: true, amount: 0.5 }}
              transition={{
                type: "spring",
                stiffness: 160,
                damping: 24,
                delay: i * 0.08,
              }}
            >
              <Icon size={22} className={styles.icon} aria-hidden="true" />
              <div>
                <h3 className={styles.factTitle}>{title}</h3>
                <p className={styles.factBody}>{body}</p>
              </div>
            </motion.li>
          ))}
        </ul>
      </div>
    </section>
  );
}
