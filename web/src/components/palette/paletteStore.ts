import { createStore } from "../../lib/store";

export const paletteOpenStore = createStore(false);

export const openPalette = () => paletteOpenStore.set(true);
export const closePalette = () => paletteOpenStore.set(false);
