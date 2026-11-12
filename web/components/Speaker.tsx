import Butterfly from "@/components/Butterflies";
import { MarkShapes } from "@/components/Wordmark";

/**
 * who is talking, as a glyph instead of a label. astralis is its eye mark,
 * every person on the left is the butterfly (the same wing the rest of the app
 * uses for people). both sit in a fixed gutter so bubbles line up whichever
 * of them is speaking, and shrink a little on phones to give the text room
 */
const SIZE = "h-5 w-5 sm:h-6 sm:w-6";

export function AstralisMark({ breathing = false, className = "" }: {
  breathing?: boolean; className?: string;
}) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden
         className={`${SIZE} text-violet-2 ${breathing ? "mark-breathe" : ""} ${className}`}>
      <MarkShapes />
    </svg>
  );
}

export function YouMark() {
  return <Butterfly pose="still" className={`${SIZE} text-ink-3`} />;
}

/** the gutter: glyph at the top, aligned with the first line of the bubble */
export function Gutter({ children }: { children?: React.ReactNode }) {
  return <div className="mt-2.5 w-5 shrink-0 sm:mt-3 sm:w-6">{children}</div>;
}
