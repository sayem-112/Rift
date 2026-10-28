import { useEffect, useId, useRef, useState, type CSSProperties, type HTMLAttributes, type Ref } from "react";
import { useStore } from "../../lib/store";
import { glassQualityStore } from "./glassQuality";
import { createRefractionMap } from "./refraction";
import styles from "./Glass.module.css";

type GlassElement = "div" | "header" | "nav" | "footer" | "section" | "aside";

export interface GlassProps extends HTMLAttributes<HTMLElement> {
  as?: GlassElement;
  ref?: Ref<HTMLElement>;
  /** Corner radius in px; use "pill" for fully rounded ends. */
  radius?: number | "pill";
  /** Lifted layers (bars, toasts, palettes) cast a soft shadow. */
  floating?: boolean;
  /** Tint color mixed into the glass, e.g. a cover's ambient color. */
  tint?: string;
  /** Refraction strength in px of backdrop displacement at the rim. */
  refraction?: number;
}

/**
 * Liquid glass for floating layers that content scrolls beneath.
 * Every browser gets layered frosted glass; Chromium also gets real edge
 * refraction through an SVG displacement filter in backdrop-filter.
 */
export function Glass({
  as: Tag = "div",
  ref,
  radius = 22,
  floating = false,
  tint,
  refraction = 64,
  className,
  style,
  children,
  ...rest
}: GlassProps) {
  const quality = useStore(glassQualityStore);
  const localRef = useRef<HTMLElement | null>(null);
  const filterId = `glass-${useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const [map, setMap] = useState<{ url: string; width: number; height: number } | null>(null);

  const refract = quality === "refract" && refraction > 0;

  useEffect(() => {
    const el = localRef.current;
    if (!refract || !el) return;

    let frame = 0;
    const observer = new ResizeObserver(([entry]) => {
      if (!entry) return;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const box = entry.borderBoxSize[0];
        const width = Math.round(box?.inlineSize ?? el.offsetWidth);
        const height = Math.round(box?.blockSize ?? el.offsetHeight);
        if (width < 8 || height < 8) return;
        const r = radius === "pill" ? height / 2 : radius;
        const bezel = Math.min(30, height / 2, width / 2);
        setMap({ url: createRefractionMap({ width, height, radius: r, bezel }), width, height });
      });
    });
    observer.observe(el);
    return () => {
      observer.disconnect();
      cancelAnimationFrame(frame);
    };
  }, [refract, radius]);

  const setRefs = (node: HTMLElement | null) => {
    localRef.current = node;
    if (typeof ref === "function") ref(node);
    else if (ref) ref.current = node;
  };

  const vars: Record<string, string> = {
    "--glass-radius": radius === "pill" ? "999px" : `${radius}px`,
  };
  if (tint) vars["--glass-tint"] = tint;
  if (refract && map) vars["--glass-refract"] = `url(#${filterId})`;

  return (
    <Tag
      ref={setRefs}
      className={[styles.glass, floating && styles.floating, className].filter(Boolean).join(" ")}
      style={{ ...vars, ...style } as CSSProperties}
      data-refract={refract && map ? "" : undefined}
      data-shape={radius === "pill" ? "pill" : undefined}
      {...rest}
    >
      {refract && map && (
        <svg className={styles.defs} aria-hidden="true" focusable="false">
          <filter
            id={filterId}
            x="0"
            y="0"
            width={map.width}
            height={map.height}
            filterUnits="userSpaceOnUse"
            primitiveUnits="userSpaceOnUse"
            colorInterpolationFilters="sRGB"
          >
            <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="blur" />
            <feImage
              href={map.url}
              x="0"
              y="0"
              width={map.width}
              height={map.height}
              preserveAspectRatio="none"
              result="map"
            />
            <feDisplacementMap in="blur" in2="map" scale={refraction} xChannelSelector="R" yChannelSelector="G" result="bent" />
            <feColorMatrix in="bent" type="saturate" values="1.9" />
          </filter>
        </svg>
      )}
      {children}
    </Tag>
  );
}
