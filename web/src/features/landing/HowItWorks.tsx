import { CheckCircle, CloudArrowUp, FilePdf } from "@phosphor-icons/react";
import { AnimatePresence, motion } from "motion/react";
import { useState, type ReactNode } from "react";
import { Cover } from "../../components/cover/Cover";
import { Glass } from "../../components/glass/Glass";
import { ProgressRing } from "../../components/Progress";
import type { Book, Device } from "../../data/types";
import { ContinueCard } from "../library/ContinueCard";
import styles from "./HowItWorks.module.css";
import layout from "./Landing.module.css";

interface Step {
  title: string;
  body: string;
  visual: ReactNode;
}

const STEPS: Step[] = [
  {
    title: "Add a book once.",
    body: "Drop in a PDF or EPUB you already own, up to 50 MB. It's encrypted and stored with your account, and the same file is never stored twice.",
    visual: <AddVisual />,
  },
  {
    title: "It's on every device.",
    body: "Sign in on your phone, your PC, your tablet. Your library is already there, and books download when you open them.",
    visual: <DevicesVisual />,
  },
  {
    title: "It opens where you stopped.",
    body: "Close the book on one device and open it on another. rift jumps to your page and tells you where it came from, with one tap to undo.",
    visual: <ContinueVisual />,
  },
];

const spring = { type: "spring", stiffness: 160, damping: 24 } as const;

export function HowItWorks() {
  const [active, setActive] = useState(0);

  return (
    <section id="how" className={`${layout.section} ${layout.container}`} aria-labelledby="how-title">
      <h2 id="how-title" className={layout.heading}>
        Set it up once. Then just read.
      </h2>

      <div className={styles.story}>
        <ol className={styles.steps}>
          {STEPS.map((step, i) => (
            <motion.li
              key={step.title}
              className={styles.step}
              data-active={active === i || undefined}
              onViewportEnter={() => setActive(i)}
              viewport={{ amount: 0.6 }}
            >
              <span className={`${styles.index} tabular`} aria-hidden="true">
                {i + 1}
              </span>
              <div>
                <h3 className={styles.stepTitle}>{step.title}</h3>
                <p className={styles.stepBody}>{step.body}</p>
              </div>
              {/* On narrow screens each step carries its own visual. */}
              <div className={styles.inlineVisual} aria-hidden="true">
                {step.visual}
              </div>
            </motion.li>
          ))}
        </ol>

        <div className={styles.stickyCol} aria-hidden="true">
          <div className={styles.sticky}>
            <AnimatePresence mode="wait">
              <motion.div
                key={active}
                className={styles.visualFrame}
                initial={{ opacity: 0, y: 24, scale: 0.98 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                exit={{
                  opacity: 0,
                  y: -16,
                  scale: 0.98,
                  transition: { duration: 0.2 },
                }}
                transition={spring}
              >
                {STEPS[active]!.visual}
              </motion.div>
            </AnimatePresence>
          </div>
        </div>
      </div>
    </section>
  );
}

/*
 * The visuals play when they come into view: in the sticky column that is as
 * soon as their step is active, and on narrow screens as each one scrolls in.
 */
const enter = { once: true, amount: 0.4 } as const;

/* Step 1: a file joins the library and uploads. */
function AddVisual() {
  return (
    <div className={styles.visual}>
      <motion.div
        className={styles.fileChip}
        initial={{ y: -36, opacity: 0, rotate: -6 }}
        whileInView={{ y: 0, opacity: 1, rotate: 0 }}
        viewport={enter}
        transition={{ ...spring, delay: 0.15 }}
      >
        <FilePdf size={18} weight="fill" />
        <span>on-the-origin-of-species.pdf</span>
        <span className={styles.fileSize}>38.5 MB</span>
      </motion.div>
      <div className={styles.miniGrid}>
        {(["meditations", "moby", "odyssey", "walden", "dracula", "middlemarch", "frankenstein", "pride", "art-of-war"] as const).map(
          (id) => (
            <Cover key={id} title={id} format="epub" edition={id} />
          ),
        )}
        <motion.div
          className={styles.newCover}
          initial={{ scale: 0.6, opacity: 0 }}
          whileInView={{ scale: 1, opacity: 1 }}
          viewport={enter}
          transition={{ ...spring, delay: 0.55 }}
        >
          <Cover title="On the Origin of Species" author="Charles Darwin" format="pdf" edition="origin" />
          <motion.span
            className={styles.uploading}
            initial={{ opacity: 1 }}
            whileInView={{ opacity: 0 }}
            viewport={enter}
            transition={{ delay: 2.2, duration: 0.4 }}
          >
            <ProgressRing label="Uploading" size={34} />
            <CloudArrowUp size={14} weight="bold" className={styles.uploadIcon} />
          </motion.span>
        </motion.div>
      </div>
    </div>
  );
}

/*
 * Step 2: the same shelf on a laptop, a tablet and a phone. The covers land
 * on all three at once, one by one, so it reads as one library, not three.
 */
const SHELF = ["meditations", "moby", "odyssey", "dracula", "walden", "pride"] as const;
const DEVICES = [
  { kind: "laptop", name: "Laptop" },
  { kind: "tablet", name: "Tablet" },
  { kind: "phone", name: "Phone" },
] as const;

function DevicesVisual() {
  return (
    <div className={`${styles.visual} ${styles.devicesVisual}`}>
      <div className={styles.devices}>
        {DEVICES.map((d) => (
          <figure key={d.kind} className={styles.device} data-kind={d.kind}>
            <div className={styles.screen}>
              <div className={styles.shelf}>
                {SHELF.map((id, i) => (
                  <motion.div
                    key={id}
                    initial={{ opacity: 0, y: 10, scale: 0.9 }}
                    whileInView={{ opacity: 1, y: 0, scale: 1 }}
                    viewport={enter}
                    transition={{ ...spring, delay: 0.15 + i * 0.09 }}
                  >
                    <Cover title={id} format="epub" edition={id} />
                  </motion.div>
                ))}
              </div>
            </div>
            <figcaption className={styles.deviceName}>
              <motion.span
                className={styles.synced}
                initial={{ opacity: 0, scale: 0.6 }}
                whileInView={{ opacity: 1, scale: 1 }}
                viewport={enter}
                transition={{ ...spring, delay: 0.8 }}
              >
                <CheckCircle size={14} weight="fill" />
              </motion.span>
              {d.name}
            </figcaption>
          </figure>
        ))}
      </div>
    </div>
  );
}

/* Step 3: the real Continue reading card, as it appears in the app. */
const now = Date.now();
const SAMPLE_DEVICES: Device[] = [
  {
    id: "phone",
    name: "Phone",
    kind: "phone",
    client: "",
    lastSeenAt: new Date(now),
    isCurrent: false,
  },
];
const SAMPLE_BOOK: Book = {
  id: "landing-meditations",
  title: "Meditations",
  author: "Marcus Aurelius",
  format: "epub",
  cover: "meditations",
  sizeBytes: 900_000,
  addedAt: new Date(now - 864e5 * 30),
  position: {
    progress: 0.62,
    label: "Book VII",
    deviceId: "phone",
    readAt: new Date(now - 2 * 36e5),
  },
  downloaded: true,
  sync: { kind: "synced" },
};

function ContinueVisual() {
  return (
    <div className={`${styles.visual} ${styles.continue}`} inert>
      <ContinueCard book={SAMPLE_BOOK} devices={SAMPLE_DEVICES} />
      <motion.div
        className={styles.jumpToast}
        initial={{ opacity: 0, y: 14 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={enter}
        transition={{ ...spring, delay: 0.5 }}
      >
        <Glass radius={16} floating refraction={0} className={styles.toast}>
          <span>
            Jumped to <strong>Book VII</strong> from your Phone
          </span>
          <span className={styles.undo}>Undo</span>
        </Glass>
      </motion.div>
    </div>
  );
}
