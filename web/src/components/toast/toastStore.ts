import { createStore } from "../../lib/store";

export interface ToastAction {
  label: string;
  /** In-app path to navigate to. */
  href?: string;
  onClick?: () => void;
}

export interface Toast {
  id: number;
  tone: "neutral" | "danger";
  message: string;
  action?: ToastAction;
}

const MAX_VISIBLE = 3;
const DURATION_MS = 6000;

export const toastStore = createStore<Toast[]>([]);
let nextId = 1;
const timers = new Map<number, ReturnType<typeof setTimeout>>();

export function toast(input: Omit<Toast, "id">): number {
  const id = nextId++;
  toastStore.set((prev) => [...prev, { ...input, id }].slice(-MAX_VISIBLE));
  timers.set(
    id,
    setTimeout(() => dismissToast(id), DURATION_MS),
  );
  return id;
}

export function dismissToast(id: number): void {
  clearTimeout(timers.get(id));
  timers.delete(id);
  toastStore.set((prev) => prev.filter((t) => t.id !== id));
}

/** Hovering or focusing a toast keeps it on screen. */
export function holdToast(id: number): void {
  clearTimeout(timers.get(id));
}

export function releaseToast(id: number): void {
  clearTimeout(timers.get(id));
  timers.set(
    id,
    setTimeout(() => dismissToast(id), DURATION_MS / 2),
  );
}
