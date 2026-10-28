import { ArrowCircleDown, Cloud } from "@phosphor-icons/react";
import styles from "./DeviceState.module.css";

/** Whether a book's file is on this device or only in the cloud; always shown, so the state scans either way. */
export function DeviceState({ downloaded, size = 15 }: { downloaded: boolean; size?: number }) {
  return downloaded ? (
    <ArrowCircleDown className={styles.local} size={size} weight="fill" role="img" aria-label="On this device" />
  ) : (
    <Cloud className={styles.cloud} size={size} role="img" aria-label="In the cloud, not downloaded" />
  );
}
