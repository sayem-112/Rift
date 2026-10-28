import { describe, expect, it } from "vitest";
import { fuzzyMatch } from "./fuzzy";

describe("fuzzyMatch", () => {
  it("matches initials across words", () => {
    expect(fuzzyMatch("sicp", "Structure and Interpretation of Computer Programs")).toBeDefined();
  });

  it("rejects out-of-order characters", () => {
    expect(fuzzyMatch("pics", "Structure and Interpretation of Computer Programs")).toBeUndefined();
  });

  it("ranks contiguous prefix matches above scattered ones", () => {
    const a = fuzzyMatch("mar", "Marcus Aurelius");
    const b = fuzzyMatch("mar", "Middlemarch");
    expect(a && b).toBeTruthy();
    expect(a!.score).toBeGreaterThan(b!.score);
  });

  it("returns highlight indices", () => {
    expect(fuzzyMatch("wal", "Walden")?.indices).toEqual([0, 1, 2]);
  });
});
