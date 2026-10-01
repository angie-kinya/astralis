import type { CSSProperties } from "react";

/**
 * The butterfly as an element, not as texture.
 *
 * GlyphField scatters this same outline across the background as texture;
 * here it is drawn large and placed on purpose, and the pose is the meaning:
 *
 *   hover  someone is working on your reply — alive, unhurried. Used while
 *          Astralis composes, instead of a spinner or three bouncing dots.
 *   rest   nothing is happening yet, and that is fine. Empty states only:
 *          an open thread, an empty notepad, the first line of a conversation.
 *   still  no opinion of its own; the caller drives it inline. The breathing
 *          pacer folds and opens the wings on the breath's own clock — the
 *          soothing intent made literal rather than decorative.
 *
 * The geometry lives here and GlyphField imports it, so the tiny drifting
 * glyphs and the perched ones are provably the same shape.
 *
 * Motion is CSS (globals.css, .bfly): the figure scales horizontally toward
 * the body, and the stroke is non-scaling so the line stays hairline as the
 * wings fold. Reduced motion freezes every pose through the global rule.
 */

/** Forewing and hindwing of the right-hand half, in a 24×24 box. */
export const GLYPH_HALF = [
  "M12.4 9.6C13.9 7 17 5 19.9 5.1C21.3 5.2 21.5 6.6 20.8 8.2C20 10.2 17.4 11.8 12.4 12.2Z",
  "M12.4 12.9C15.4 12.8 18 13.6 18.4 15.4C18.7 16.8 17.6 17.9 16.4 18.1C16.6 19.1 16.3 19.9 15.7 20C14.4 19.4 13.1 17.4 12.4 14.4Z",
] as const;

/** The body line and two short antennae. */
export const GLYPH_BODY =
  "M12 8.4V17.2M11.7 8.3C11.2 6.6 10.4 5.5 9.3 5.1M12.3 8.3C12.8 6.6 13.6 5.5 14.7 5.1";

/** Mirrors any half across the body at x=12. */
export const MIRROR = "matrix(-1 0 0 1 24 0)";

export type ButterflyPose = "hover" | "rest" | "still";

export default function Butterfly({ pose = "rest", className = "", style }: {
  pose?: ButterflyPose;
  /** Sizing and colour: `h-5 w-5 text-violet-2/70`, as for any glyph. */
  className?: string;
  /** Inline transforms only — the pacer's breath drives a `scaleX` here. */
  style?: CSSProperties;
}) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden style={style}
         className={`bfly bfly--${pose} ${className}`}>
      <g>{GLYPH_HALF.map((d) => <path key={d} d={d} />)}</g>
      <g transform={MIRROR}>{GLYPH_HALF.map((d) => <path key={d} d={d} />)}</g>
      <path d={GLYPH_BODY} />
    </svg>
  );
}
