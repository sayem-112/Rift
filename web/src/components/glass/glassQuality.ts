import { isChromium } from "../../lib/platform";
import { createStore } from "../../lib/store";

/**
 * How much glass this device gets:
 * - "refract": frosted glass plus SVG edge refraction (Chromium, capable hardware)
 * - "frost":   frosted glass only (Safari, Firefox, modest hardware)
 * - "solid":   opaque Mocha surfaces (reduced transparency, or the device dropped frames)
 */
export type GlassQuality = "refract" | "frost" | "solid";

const SESSION_KEY = "rift.glassSolid";

function initialQuality(): GlassQuality {
  if (window.matchMedia("(prefers-reduced-transparency: reduce)").matches) return "solid";
  try {
    if (sessionStorage.getItem(SESSION_KEY) === "1") return "solid";
  } catch {
    // ignore
  }
  const nav = navigator as Navigator & { deviceMemory?: number };
  const modest = (nav.deviceMemory ?? 8) <= 4 || (nav.hardwareConcurrency ?? 8) <= 2;
  return isChromium && !modest ? "refract" : "frost";
}

export const glassQualityStore = createStore<GlassQuality>(initialQuality());

function apply() {
  document.documentElement.dataset.glass = glassQualityStore.get();
}
apply();
glassQualityStore.subscribe(apply);

window.matchMedia("(prefers-reduced-transparency: reduce)").addEventListener("change", (e) => {
  glassQualityStore.set(e.matches ? "solid" : initialQuality());
});

/*
 * Frame-rate guard: sample frame times during the first real scroll. If the
 * median frame misses ~36fps, blur is too expensive here, so drop to solid
 * surfaces for the rest of the session.
 */
const SAMPLE_FRAMES = 45;
const SLOW_FRAME_MS = 28;
let sampling = false;

function sampleFrames() {
  if (sampling || glassQualityStore.get() === "solid") return;
  sampling = true;
  const times: number[] = [];
  let last = performance.now();
  const tick = (now: number) => {
    times.push(now - last);
    last = now;
    if (times.length < SAMPLE_FRAMES) {
      requestAnimationFrame(tick);
      return;
    }
    times.sort((a, b) => a - b);
    const median = times[Math.floor(times.length / 2)] ?? 0;
    if (median > SLOW_FRAME_MS) {
      glassQualityStore.set("solid");
      try {
        sessionStorage.setItem(SESSION_KEY, "1");
      } catch {
        // ignore
      }
    }
  };
  requestAnimationFrame(tick);
}

document.addEventListener("scroll", sampleFrames, { capture: true, passive: true, once: true });

/*
 * Scroll-linked sheen: expose the latest scroll offset so the glass highlight
 * drifts slightly as content moves beneath it.
 */
let frame = 0;
document.addEventListener(
  "scroll",
  (event) => {
    if (frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      const target = event.target;
      const top = target instanceof Element ? target.scrollTop : window.scrollY;
      document.documentElement.style.setProperty("--glass-scroll", `${Math.round(top)}px`);
    });
  },
  { capture: true, passive: true },
);
