import { X } from "@phosphor-icons/react";
import { AnimatePresence, motion, useInView, useReducedMotion } from "motion/react";
import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { Cover } from "../../components/cover/Cover";
import { Glass } from "../../components/glass/Glass";
import { ProgressBar } from "../../components/Progress";
import styles from "./HandoffDemo.module.css";

/*
 * The landing page's signature: the product's one trick, performed.
 * Someone reads Meditations on a phone and turns the page. The phone goes
 * down, the place they stopped lifts off it as a bookmark, lands on the same
 * book in the laptop's library, and the laptop opens at that page with the
 * app's real toast.
 *
 * The text is Marcus Aurelius' Meditations, Book VII, in George Long's 1862
 * translation (public domain).
 */

const PAGES = {
  119: [
    {
      n: 59,
      text: "Look within. Within is the fountain of good, and it will ever bubble up, if thou wilt ever dig.",
    },
    {
      n: 60,
      text: "The body ought to be compact, and to show no irregularity either in motion or attitude. For what the mind shows in the face by maintaining in it the expression of intelligence and propriety, that ought to be required also in the whole body. But all these things should be observed without affectation.",
    },
    {
      n: 61,
      text: "The art of life is more like the wrestler's art than the dancer's, in respect of this, that it should stand ready and firm to meet onsets which are sudden and unexpected.",
    },
  ],
  120: [
    {
      n: 62,
      text: "Constantly observe who those are whose approbation thou wishest to have, and what ruling principles they possess. For then thou wilt neither blame those who offend involuntarily, nor wilt thou want their approbation, if thou lookest to the sources of their opinions and appetites.",
    },
    {
      n: 63,
      text: "Every soul, the philosopher says, is involuntarily deprived of truth; consequently in the same way it is deprived of justice and temperance and benevolence and everything of the kind.",
    },
  ],
} as const;

const TOTAL_PAGES = 190;
const SHELF = ["meditations", "moby", "odyssey", "walden", "origin", "dracula", "middlemarch", "frankenstein", "pride", "art-of-war"];

type Point = { x: number; y: number };
type Phase = "read" | "turn" | "lift" | "travel" | "open";

/** [phase, how long to hold it in ms] */
const TIMELINE: [Phase, number][] = [
  ["read", 2200],
  ["turn", 1900],
  ["lift", 750],
  ["travel", 1050],
  ["open", 4600],
];

/* Loop a few times while it's on screen, then rest on the finished handoff. */
const MAX_LOOPS = 3;

const spring = { type: "spring", stiffness: 260, damping: 30 } as const;
const soft = { type: "spring", stiffness: 140, damping: 24 } as const;
const glide = { type: "spring", stiffness: 70, damping: 16, mass: 1 } as const;

export function HandoffDemo() {
  const reduce = useReducedMotion();
  const ref = useRef<HTMLElement>(null);
  const inView = useInView(ref, { amount: 0.35 });
  const sceneRef = useRef<HTMLDivElement>(null);
  const phoneRef = useRef<HTMLDivElement>(null);
  const targetRef = useRef<HTMLDivElement>(null);
  const [step, setStep] = useState(0);
  const [loops, setLoops] = useState(0);
  const [flight, setFlight] = useState<{ from: Point; to: Point } | null>(null);

  const resting = reduce || loops >= MAX_LOOPS;
  const phase: Phase = resting ? "open" : TIMELINE[step]![0];

  useEffect(() => {
    if (resting || !inView) return;
    const timer = setTimeout(() => {
      const next = (step + 1) % TIMELINE.length;
      if (next === 0 && loops + 1 >= MAX_LOOPS) {
        setLoops(MAX_LOOPS);
        return;
      }
      if (next === 0) setLoops((l) => l + 1);
      setStep(next);
    }, TIMELINE[step]![1]);
    return () => clearTimeout(timer);
  }, [step, loops, resting, inView]);

  // The bookmark flies in its own layer above both devices, so measure where
  // it lifts off (upper middle of the phone's page) and where it lands (the
  // Meditations cover) relative to the scene, just before it lifts.
  useLayoutEffect(() => {
    if (phase !== "lift") return;
    const scene = sceneRef.current?.getBoundingClientRect();
    const phone = phoneRef.current?.getBoundingClientRect();
    const target = targetRef.current?.getBoundingClientRect();
    if (!scene || !phone || !target) return;
    setFlight({
      from: {
        x: phone.left + phone.width / 2 - scene.left,
        y: phone.top + phone.height * 0.4 - scene.top,
      },
      to: {
        x: target.left + target.width / 2 - scene.left,
        y: target.top + target.height * 0.56 - scene.top,
      },
    });
  }, [phase]);

  const page = phase === "read" ? 119 : 120;
  const flying = phase === "lift" || phase === "travel" ? flight : null;
  const down = phase === "lift" || phase === "travel" || phase === "open";
  const opened = phase === "open";

  return (
    <figure ref={ref} className={styles.stage}>
      <figcaption className="visually-hidden">
        Animation: someone reading Meditations on a phone turns to page 120 and puts the phone down. A bookmark lifts off the phone, lands
        on Meditations in a laptop's library, and the laptop opens the book at page 120 with the message “Jumped to p. 120 from your Phone.”
      </figcaption>

      <div ref={sceneRef} className={styles.scene} aria-hidden="true">
        {/* The laptop, behind. */}
        <div className={styles.desktop}>
          <div className={styles.titlebar}>
            <span className={styles.titleDots}>
              <i />
              <i />
              <i />
            </span>
          </div>
          <div className={styles.desktopBody}>
            <div className={styles.sidebar}>
              <span className={styles.sideBrand} />
              <span className={styles.sideItem} data-active="" />
              <span className={styles.sideItem} />
              <span className={styles.sideItem} />
            </div>

            <div className={styles.main}>
              <AnimatePresence mode="popLayout" initial={false}>
                {opened ? (
                  <motion.div
                    key="reader"
                    className={styles.reader}
                    initial={{ opacity: 0, scale: 0.94, y: 10 }}
                    animate={{ opacity: 1, scale: 1, y: 0 }}
                    exit={{ opacity: 0, transition: { duration: 0.3 } }}
                    transition={soft}
                  >
                    <div className={styles.spread}>
                      <BookPage page={119} className={styles.desktopPage} />
                      <BookPage page={120} className={styles.desktopPage}>
                        <motion.span
                          className={styles.ribbon}
                          initial={{ scaleY: 0 }}
                          animate={{ scaleY: 1 }}
                          transition={{ ...spring, delay: 0.3 }}
                        />
                      </BookPage>
                    </div>
                  </motion.div>
                ) : (
                  <motion.div
                    key="library"
                    className={styles.library}
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{
                      opacity: 0,
                      scale: 1.03,
                      transition: { duration: 0.3 },
                    }}
                  >
                    <p className={styles.libraryHead}>Library</p>
                    <div className={styles.shelf}>
                      {SHELF.map((id) => (
                        <div
                          key={id}
                          ref={id === "meditations" ? targetRef : undefined}
                          className={styles.shelfCover}
                          data-target={id === "meditations" && phase === "travel" ? "" : undefined}
                        >
                          <Cover title={id} format="epub" edition={id} />
                        </div>
                      ))}
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>

              <AnimatePresence>
                {opened && (
                  <motion.div
                    className={styles.toastWrap}
                    initial={{ opacity: 0, y: 16, scale: 0.96 }}
                    animate={{ opacity: 1, y: 0, scale: 1 }}
                    exit={{ opacity: 0, y: 8 }}
                    transition={{ ...spring, delay: 0.45 }}
                  >
                    <Glass radius={14} refraction={0} floating className={styles.toast}>
                      <span>
                        Jumped to <strong className="tabular">p. 120</strong> from your Phone
                      </span>
                      <span className={styles.undo}>Undo</span>
                      <X size={11} weight="bold" className={styles.toastClose} />
                    </Glass>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>
        </div>

        {/* The phone, in front. */}
        <motion.div ref={phoneRef} className={styles.phone} animate={{ y: down ? 6 : 0 }} transition={soft}>
          <div className={styles.phoneScreen}>
            <span className={styles.island} />
            <AnimatePresence mode="popLayout" initial={false}>
              <motion.div
                key={page}
                className={styles.phonePageWrap}
                initial={{ x: "100%" }}
                animate={{ x: 0 }}
                exit={{ x: "-30%", opacity: 0.4 }}
                transition={{ type: "spring", stiffness: 210, damping: 30 }}
              >
                <BookPage page={page} className={styles.phonePage} />
              </motion.div>
            </AnimatePresence>

            <motion.div className={styles.dim} animate={{ opacity: down ? 0.3 : 0 }} transition={{ duration: 0.5 }} />
          </div>
        </motion.div>

        {/* The bookmark, above both devices: it lifts off the phone and lands on the cover. */}
        <AnimatePresence>
          {flying && (
            <motion.span
              key="bookmark"
              className={styles.flight}
              initial={{
                x: flying.from.x,
                y: flying.from.y + 14,
                opacity: 0,
                scale: 0.9,
              }}
              animate={
                phase === "lift"
                  ? {
                      x: flying.from.x,
                      y: flying.from.y,
                      opacity: 1,
                      scale: 1,
                      transition: spring,
                    }
                  : {
                      x: flying.to.x,
                      y: flying.to.y,
                      opacity: 1,
                      scale: 1,
                      transition: glide,
                    }
              }
              exit={{ opacity: 0, scale: 0.8, transition: { duration: 0.2 } }}
            >
              <Bookmark />
            </motion.span>
          )}
        </AnimatePresence>
      </div>
    </figure>
  );
}

/* A page as the reader lays it out: running head, numbered sections, folio. */
function BookPage({ page, className, children }: { page: 119 | 120; className?: string; children?: ReactNode }) {
  return (
    <div className={`${styles.page} ${className ?? ""}`}>
      {children}
      <p className={styles.runningHead}>
        <span>Book VII</span>
        <span>Meditations</span>
      </p>
      <div className={styles.text}>
        {PAGES[page].map((s) => (
          <p key={s.n}>
            <span className={styles.sectionNo}>{s.n}.</span> {s.text}
          </p>
        ))}
      </div>
      <div className={styles.folio}>
        <ProgressBar value={page / TOTAL_PAGES} label="Reading progress" />
        <span className="tabular">
          {page} / {TOTAL_PAGES}
        </span>
      </div>
    </div>
  );
}

/* The place you stopped, as a small card that travels between devices. */
function Bookmark() {
  return (
    <Glass radius={12} refraction={0} floating className={styles.bookmark}>
      <span className={styles.bookmarkCover}>
        <Cover title="Meditations" format="epub" edition="meditations" />
      </span>
      <span className={styles.bookmarkText}>
        <strong>Meditations</strong>
        <span className="tabular">p. 120</span>
      </span>
    </Glass>
  );
}
