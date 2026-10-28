const MB = 1024 * 1024;

/** "143 MB", "4.2 MB", "820 KB" */
export function formatBytes(bytes: number): string {
  if (bytes >= 10 * MB) return `${Math.round(bytes / MB)} MB`;
  if (bytes >= MB) return `${(bytes / MB).toFixed(1).replace(/\.0$/, "")} MB`;
  return `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto", style: "long" });

/** "just now", "2 hours ago", "yesterday", "3 weeks ago" */
export function formatRelative(date: Date, now: Date = new Date()): string {
  const seconds = Math.round((date.getTime() - now.getTime()) / 1000);
  const abs = Math.abs(seconds);
  if (abs < 45) return "just now";
  if (abs < 3600) return rtf.format(Math.round(seconds / 60), "minute");
  if (abs < 86400) return rtf.format(Math.round(seconds / 3600), "hour");
  if (abs < 86400 * 7) return rtf.format(Math.round(seconds / 86400), "day");
  if (abs < 86400 * 30) return rtf.format(Math.round(seconds / (86400 * 7)), "week");
  return rtf.format(Math.round(seconds / (86400 * 30)), "month");
}

/** 0.6234 -> "62%" */
export function formatPercent(fraction: number): string {
  return `${Math.round(Math.min(1, Math.max(0, fraction)) * 100)}%`;
}

/** "the-art_of-war.pdf" -> "The art of war" */
export function titleFromFilename(filename: string): string {
  const base = filename.replace(/\.[^.]+$/, "").replace(/[_-]+/g, " ").replace(/\s+/g, " ").trim();
  if (!base) return "Untitled";
  return base.charAt(0).toUpperCase() + base.slice(1);
}
