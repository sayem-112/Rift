interface NavigatorUAData {
  brands?: { brand: string }[];
  platform?: string;
}

const uaData = (navigator as Navigator & { userAgentData?: NavigatorUAData }).userAgentData;

/** Chromium can run SVG filters inside backdrop-filter; Safari and Firefox cannot. */
export const isChromium: boolean = uaData?.brands?.some((b) => b.brand === "Chromium") ?? false;

export const isApple: boolean = /Mac|iPhone|iPad/.test(uaData?.platform ?? navigator.platform ?? "");

/** Label for the command palette shortcut. */
export const modKeyLabel = isApple ? "⌘" : "Ctrl";

export function prefersReducedMotion(): boolean {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
