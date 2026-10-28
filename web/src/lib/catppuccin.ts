/** Catppuccin Mocha accents, in the palette's canonical order. */
export const ACCENTS = [
  { id: "rosewater", label: "Rosewater", hex: "#f5e0dc" },
  { id: "flamingo", label: "Flamingo", hex: "#f2cdcd" },
  { id: "pink", label: "Pink", hex: "#f5c2e7" },
  { id: "mauve", label: "Mauve", hex: "#cba6f7" },
  { id: "red", label: "Red", hex: "#f38ba8" },
  { id: "maroon", label: "Maroon", hex: "#eba0ac" },
  { id: "peach", label: "Peach", hex: "#fab387" },
  { id: "yellow", label: "Yellow", hex: "#f9e2af" },
  { id: "green", label: "Green", hex: "#a6e3a1" },
  { id: "teal", label: "Teal", hex: "#94e2d5" },
  { id: "sky", label: "Sky", hex: "#89dceb" },
  { id: "sapphire", label: "Sapphire", hex: "#74c7ec" },
  { id: "blue", label: "Blue", hex: "#89b4fa" },
  { id: "lavender", label: "Lavender", hex: "#b4befe" },
] as const;

export type AccentId = (typeof ACCENTS)[number]["id"];

export const DEFAULT_ACCENT: AccentId = "mauve";

export function accentHex(id: AccentId): string {
  return ACCENTS.find((a) => a.id === id)?.hex ?? "#cba6f7";
}

export function isAccentId(value: unknown): value is AccentId {
  return typeof value === "string" && ACCENTS.some((a) => a.id === value);
}
