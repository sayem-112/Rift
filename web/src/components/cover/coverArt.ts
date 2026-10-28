import { ACCENTS } from "../../lib/catppuccin";
import { hashString } from "../../lib/hash";
import { EDITIONS } from "./editions";

export interface CoverArt {
  from: string;
  to: string;
  angle: number;
  /** Horizontal position of the soft highlight, in %. */
  glint: number;
}

/**
 * Generated covers blend two neighboring Mocha accents (mauve->red,
 * teal->sky, ...) so every pairing stays calm. The title picks the pair, its
 * direction and the angle, so a book always gets the same cover and
 * neighbors in the grid rarely look alike (14 pairs x 2 directions).
 */
export function coverArt(title: string): CoverArt {
  const hash = hashString(title.trim().toLowerCase());
  const i = hash % ACCENTS.length;
  const a = ACCENTS[i]!.hex;
  const b = ACCENTS[(i + 1) % ACCENTS.length]!.hex;
  const reversed = ((hash >>> 5) & 1) === 1;
  const angle = 125 + ((hash >>> 8) % 70);
  const glint = (hash >>> 16) % 90;
  return reversed ? { from: b, to: a, angle, glint } : { from: a, to: b, angle, glint };
}

/** The two colors of a book's cover, for light cast behind it. */
export function coverGlow(title: string, edition?: string): { from: string; to: string; angle: number } {
  const art = coverArt(title);
  const own = edition ? EDITIONS[edition] : undefined;
  return own ? { from: own.glow[0], to: own.glow[1], angle: art.angle } : art;
}
