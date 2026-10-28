export type BookFormat = "pdf" | "epub";

export type DeviceKind = "phone" | "tablet" | "desktop";

export interface Device {
  id: string;
  name: string;
  kind: DeviceKind;
  /** Browser or app the device signed in with, e.g. "Chrome on Android". */
  client: string;
  lastSeenAt: Date;
  isCurrent: boolean;
}

export interface ReadingPosition {
  /** 0..1 through the book. */
  progress: number;
  /** Human label for the spot, e.g. "p. 120 of 340" or "Chapter 7". */
  label: string;
  deviceId: string;
  readAt: Date;
}

export type SyncState =
  | { kind: "synced" }
  /** Added on this device; waiting for the sync API to accept the upload. */
  | { kind: "queued" };

export interface Book {
  id: string;
  title: string;
  author?: string;
  format: BookFormat;
  /** Cover art carried by the book itself; absent means a generated cover. */
  cover?: string;
  sizeBytes: number;
  addedAt: Date;
  /** Latest position across all devices; undefined if never opened. */
  position?: ReadingPosition;
  /** Manual override; undefined means "derive from progress". */
  finished?: boolean;
  /** Whether the encrypted file is cached on this device. */
  downloaded: boolean;
  sync: SyncState;
}
