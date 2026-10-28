import { flushSync } from "react-dom";
import { prefersReducedMotion } from "./platform";

/**
 * Run a React state update inside a View Transition so named elements
 * (book covers) morph to their new positions. Falls back to a plain update.
 */
export function withViewTransition(update: () => void): void {
  if (!document.startViewTransition || prefersReducedMotion()) {
    update();
    return;
  }
  document.startViewTransition(() => {
    flushSync(update);
  });
}
