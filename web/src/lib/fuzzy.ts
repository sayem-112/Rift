export interface FuzzyMatch {
  score: number;
  /** Indices of matched characters in the target, for highlighting. */
  indices: number[];
}

/**
 * Subsequence match with bonuses for word starts and consecutive runs, so
 * "sicp" finds "Structure and Interpretation of Computer Programs" and "med"
 * ranks "Meditations" above "Middlemarch". Returns undefined when not all
 * query characters appear in order.
 */
export function fuzzyMatch(query: string, target: string): FuzzyMatch | undefined {
  const q = query.trim().toLowerCase();
  if (!q) return { score: 0, indices: [] };
  const t = target.toLowerCase();

  const indices: number[] = [];
  let score = 0;
  let ti = 0;
  let prev = -2;

  for (const ch of q) {
    if (ch === " ") continue;
    let found = -1;
    // Prefer the next word-start occurrence; fall back to the next occurrence.
    for (let j = ti; j < t.length; j++) {
      if (t[j] === ch && (j === 0 || /[\s\-_.:(&]/.test(t[j - 1]!))) {
        found = j;
        break;
      }
    }
    const next = t.indexOf(ch, ti);
    if (found === -1 || (next !== -1 && next === prev + 1)) found = next;
    if (found === -1) return undefined;

    const wordStart = found === 0 || /[\s\-_.:(&]/.test(t[found - 1]!);
    score += 1;
    if (wordStart) score += 3;
    if (found === prev + 1) score += 2;
    if (found === 0) score += 2;
    indices.push(found);
    prev = found;
    ti = found + 1;
  }

  // Shorter targets with the same matches are more relevant.
  score -= t.length * 0.01;
  return { score, indices };
}
