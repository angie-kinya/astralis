"use client";

import { MARK, MarkShapes, ORBIT_PATH, SAT, TILT_TRANSFORM } from "@/components/Wordmark";
import { useReducedMotion } from "@/lib/motion";

const ORBIT_SECONDS = 26;
// Start the moving satellite where the static one sits, so the swap after
// hydration (or to reduced motion) does not jump. The orbit path runs
// clockwise from 0°, so -42° is ~318/360 of the way round.
const BEGIN = `-${(((360 + MARK.satAngle) / 360) * ORBIT_SECONDS).toFixed(2)}s`;

/**
 * The large mark on the landing page: the same shapes as the header, with
 * the satellite slowly orbiting, a soft breathing glow, and a rare blink.
 * Reduced motion gets the still mark (the CSS animations are neutralised by
 * the global rule in globals.css; the SMIL orbit is not rendered at all).
 */
export default function HeroMark({ className = "" }: { className?: string }) {
  const reduced = useReducedMotion();
  return (
    <div aria-hidden className={`relative ${className}`}>
      <div className="hero-glow absolute -inset-[35%] rounded-full" />
      <svg viewBox="0 0 24 24" className="relative h-full w-full overflow-visible text-violet-2">
        <g className="hero-eye">
          <MarkShapes satellite={false} />
          <g transform={TILT_TRANSFORM}>
            {reduced ? (
              <circle cx={SAT.x} cy={SAT.y} r={MARK.satR} fill="currentColor" />
            ) : (
              <circle r={MARK.satR} fill="currentColor">
                <animateMotion dur={`${ORBIT_SECONDS}s`} begin={BEGIN}
                               repeatCount="indefinite" path={ORBIT_PATH} />
              </circle>
            )}
          </g>
        </g>
      </svg>
    </div>
  );
}
