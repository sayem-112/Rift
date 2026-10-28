/**
 * Builds the displacement map for liquid-glass refraction.
 *
 * The map encodes, per pixel, which way to bend the backdrop: red is the x
 * offset, green the y offset, 128 meaning "no shift". Pixels inside a bezel
 * along the rounded-rect edge pull their sample inward along the surface
 * normal, so content seen through the rim bends like light through a thick
 * curved lens. The interior stays undistorted, so text behind it stays legible.
 */

/** Render the map below 1:1; feImage stretches it and the gradient is smooth. */
const MAP_SCALE = 0.5;

export interface RefractionMapOptions {
  width: number;
  height: number;
  radius: number;
  /** Width of the refracting rim in CSS px. */
  bezel: number;
}

export function createRefractionMap({ width, height, radius, bezel }: RefractionMapOptions): string {
  const cw = Math.max(2, Math.round(width * MAP_SCALE));
  const ch = Math.max(2, Math.round(height * MAP_SCALE));
  const canvas = document.createElement("canvas");
  canvas.width = cw;
  canvas.height = ch;
  const ctx = canvas.getContext("2d");
  if (!ctx) return "";

  const image = ctx.createImageData(cw, ch);
  const data = image.data;
  const r = Math.min(radius, width / 2, height / 2);
  const halfW = width / 2;
  const halfH = height / 2;

  for (let y = 0; y < ch; y++) {
    const py = (y + 0.5) / MAP_SCALE - halfH;
    for (let x = 0; x < cw; x++) {
      const px = (x + 0.5) / MAP_SCALE - halfW;

      // Signed distance to the rounded rectangle (negative inside).
      const qx = Math.abs(px) - (halfW - r);
      const qy = Math.abs(py) - (halfH - r);
      const ox = Math.max(qx, 0);
      const oy = Math.max(qy, 0);
      const outside = Math.hypot(ox, oy);
      const depth = r - outside - Math.min(Math.max(qx, qy), 0);

      let dx = 0;
      let dy = 0;
      if (depth >= 0 && depth < bezel) {
        // Outward surface normal.
        let nx: number;
        let ny: number;
        if (qx > 0 && qy > 0 && outside > 0) {
          nx = ox / outside;
          ny = oy / outside;
        } else if (qx > qy) {
          nx = 1;
          ny = 0;
        } else {
          nx = 0;
          ny = 1;
        }
        nx *= Math.sign(px) || 1;
        ny *= Math.sign(py) || 1;

        // Convex squircle lens: the surface curves away fastest at the rim, so
        // the bend rises steeply there and flattens smoothly toward the interior.
        const t = 1 - depth / bezel;
        const strength = Math.pow(t, 2.2);
        dx = -nx * strength;
        dy = -ny * strength;
      }

      const i = (y * cw + x) * 4;
      data[i] = Math.round(128 + dx * 127);
      data[i + 1] = Math.round(128 + dy * 127);
      data[i + 2] = 128;
      data[i + 3] = 255;
    }
  }

  ctx.putImageData(image, 0, 0);
  return canvas.toDataURL("image/png");
}
