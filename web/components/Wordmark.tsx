/**
 * The mark: an eye that is also a small orrery. The orbit is the eyelid, the
 * core is the iris (with a glint cut out of it, which is what makes it read as
 * an eye rather than a planet), and the satellite rides the orbit.
 *
 * This file is the only place the geometry lives. The header, landing, sign-in
 * and the favicons (generated from markSvg() by scripts/icons.mjs) all draw
 * from these numbers, so they cannot drift apart.
 */

// All in a 24×24 box, centred at (12, 12). The orbit is drawn in a frame
// tilted by TILT degrees, so the satellite and the landing's animated orbit
// can share one path.
export const MARK = {
  rx: 10.4, ry: 5.9, tilt: -16, stroke: 1.75,
  iris: 3.7,
  // The glint: a hole in the iris, up and to the right, like reflected light.
  glint: { x: 1.3, y: -1.3, r: 1.3 },
  // Satellite position on the orbit, as an angle in the tilted frame.
  satAngle: -42, satR: 1.9,
} as const;

/** The orbit as a closed path in the tilted frame (origin at the centre). */
export const ORBIT_PATH =
  `M${MARK.rx} 0A${MARK.rx} ${MARK.ry} 0 1 1 ${-MARK.rx} 0A${MARK.rx} ${MARK.ry} 0 1 1 ${MARK.rx} 0Z`;

const rad = (MARK.satAngle * Math.PI) / 180;
export const SAT = {
  x: +(MARK.rx * Math.cos(rad)).toFixed(3),
  y: +(MARK.ry * Math.sin(rad)).toFixed(3),
};

/** Iris with the glint punched out (even-odd), so it needs no mask or id. */
const g = MARK.glint, r = MARK.iris;
export const IRIS_PATH =
  `M${12 - r} 12a${r} ${r} 0 1 0 ${2 * r} 0a${r} ${r} 0 1 0 ${-2 * r} 0Z` +
  `M${12 + g.x - g.r} ${12 + g.y}a${g.r} ${g.r} 0 1 0 ${2 * g.r} 0a${g.r} ${g.r} 0 1 0 ${-2 * g.r} 0Z`;

export const TILT_TRANSFORM = `translate(12 12) rotate(${MARK.tilt})`;

/**
 * The shapes, without the <svg>. `satellite={false}` lets the landing hero
 * draw its own moving satellite on the same orbit.
 */
export function MarkShapes({ satellite = true }: { satellite?: boolean }) {
  return (
    <>
      <g transform={TILT_TRANSFORM}>
        <path d={ORBIT_PATH} fill="none" stroke="currentColor"
              strokeWidth={MARK.stroke} opacity=".7" />
        {satellite && <circle cx={SAT.x} cy={SAT.y} r={MARK.satR} fill="currentColor" />}
      </g>
      <path d={IRIS_PATH} fill="currentColor" fillRule="evenodd" />
    </>
  );
}

/** The same mark as a standalone SVG document, for the favicon generator. */
export function markSvg({ color, style = "" }: { color: string; style?: string }): string {
  return [
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">`,
    style && `<style>${style}</style>`,
    `<g class="m" fill="${color}" stroke="${color}">`,
    `<g transform="${TILT_TRANSFORM}">`,
    `<path d="${ORBIT_PATH}" fill="none" stroke-width="${MARK.stroke}" opacity=".7"/>`,
    `<circle cx="${SAT.x}" cy="${SAT.y}" r="${MARK.satR}" stroke="none"/>`,
    `</g>`,
    `<path d="${IRIS_PATH}" fill-rule="evenodd" stroke="none"/>`,
    `</g></svg>`,
  ].filter(Boolean).join("");
}

export default function Wordmark({ withText = true, className = "" }: {
  withText?: boolean; className?: string;
}) {
  return (
    <span className={`inline-flex items-center gap-2.5 ${className}`}>
      <svg viewBox="0 0 24 24" className="h-[20px] w-[20px] text-violet-2" aria-hidden>
        <MarkShapes />
      </svg>
      {withText && (
        <span className="font-mono text-[.875rem] font-medium tracking-[-.01em] text-ink">
          astralis
        </span>
      )}
    </span>
  );
}
