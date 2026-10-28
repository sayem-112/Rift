import { useLayoutEffect, type RefObject } from "react";

/**
 * Shared-element morph for the one cover being opened.
 *
 * Only a single element may carry the name at a time: if every cover were
 * named, switching grid/list or filters would fly a dozen covers across each
 * other. So the name is attached imperatively to the clicked cover just
 * before navigation, and re-attached to that same cover when the library
 * mounts again on the way back, then cleared once the morph has run.
 */

export const COVER_TRANSITION_NAME = "book-cover";

/** Long enough for the morph (see --dur-move) to finish. */
const CLEAR_AFTER_MS = 700;

let openedBookId: string | undefined;

/** Call from the click that opens a book, before the router navigates. */
export function markOpening(bookId: string, element: HTMLElement | null): void {
  openedBookId = bookId;
  if (element) element.style.viewTransitionName = COVER_TRANSITION_NAME;
}

/** Re-attaches the name to the cover the reader is returning from. */
export function useReturningCover(bookId: string, ref: RefObject<HTMLElement | null>): void {
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || bookId !== openedBookId) return;
    el.style.viewTransitionName = COVER_TRANSITION_NAME;
    const timer = setTimeout(() => {
      el.style.viewTransitionName = "";
      if (openedBookId === bookId) openedBookId = undefined;
    }, CLEAR_AFTER_MS);
    return () => clearTimeout(timer);
  }, [bookId, ref]);
}
