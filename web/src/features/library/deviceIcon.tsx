import { DesktopTower, DeviceMobile, DeviceTablet, type Icon } from "@phosphor-icons/react";
import type { DeviceKind } from "../../data/types";

export const DEVICE_ICONS: Record<DeviceKind, Icon> = {
  phone: DeviceMobile,
  tablet: DeviceTablet,
  desktop: DesktopTower,
};
