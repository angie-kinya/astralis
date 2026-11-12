"use client";

import type { Player } from "@/lib/api";
import { BOXES, N, READING_ORDER, edgeLabel, ends } from "@/components/game/rules";

/** whose colour a player draws in: violet for you, cyan for a person, violet-2 (the eye) for astralis */
export type Tone = "you" | "them" | "bot";

// a 100 unit square: stars 26 apart with an 11 unit margin, so glows and hit areas never clip
const M = 11, S = 26;
const at = (i: number) => M + i * S;

/**
 * the star field. purely a picture of the state it is handed plus one
 * callback; it never decides whether a move is allowed, it only refuses to
 * offer one while `live` is false.
 *
 * every undrawn edge is a real control: a focusable group with a fat
 * transparent stroke as its hit area (about 34px on a phone), so a thumb or
 * a Tab key finds it as easily as a mouse
 */
export default function Board({
  drawn, owner, drawers, lastEdge, lastDrawer, tone, interactive, live, onMove,
}: {
  drawn: boolean[];
  owner: Player[];
  /** who drew each edge, 0 when unknown (older history on a fresh load) */
  drawers: Player[];
  lastEdge: number | null;
  lastDrawer: Player;
  tone: (p: Player) => Tone;
  /** false once the game is over: undrawn edges leave the tab order */
  interactive: boolean;
  /** true only when a move from this tab would be legal right now */
  live: boolean;
  onMove: (edge: number) => void;
}) {
  const touched = (r: number, c: number) =>
    drawn.some((d, e) => d && ends(e).some((s) => s.r === r && s.c === c));

  return (
    <div className="game-board mx-auto aspect-square"
         style={{ width: "min(100%, 30rem, max(15rem, calc(100dvh - 17rem)))" }}>
      <svg viewBox="0 0 100 100" role="group" aria-label="constellations board"
           className="h-full w-full overflow-visible" data-live={live}>
        {/* claimed squares sit underneath everything, a soft wash and a small mark of who holds it */}
        {Array.from({ length: BOXES }, (_, b) => {
          const p = owner[b] ?? 0;
          if (!p) return null;
          const r = Math.floor(b / 3), c = b % 3;
          const x = at(c), y = at(r), cx = x + S / 2, cy = y + S / 2;
          const t = tone(p);
          return (
            <g key={`b${b}-${p}`} className={`game-box gp-${t}`}>
              <rect x={x + 2.2} y={y + 2.2} width={S - 4.4} height={S - 4.4} rx={2} />
              {/* shape as well as colour: a four point star for you, an orbit ring for them */}
              {t === "you"
                ? <path className="game-box-mark" d={sparkle(cx, cy, 2.6)} />
                : <circle className="game-box-ring" cx={cx} cy={cy} r={1.9} />}
            </g>
          );
        })}

        {/* the soft halo under the most recent line */}
        {lastEdge !== null && drawn[lastEdge] && (
          <Line key={`halo-${lastEdge}`} e={lastEdge} className={`game-halo gp-${lastDrawer ? tone(lastDrawer) : "you"}`} />
        )}

        {READING_ORDER.map((e) => {
          if (drawn[e]) {
            const p = drawers[e] ?? 0;
            return (
              <Line key={`d${e}`} e={e} pathLength={1}
                    className={`game-line ${p ? `gp-${tone(p)}` : "game-line--old"}`} />
            );
          }
          return (
            <g key={`u${e}`} role="button" className="game-edge"
               tabIndex={interactive ? 0 : -1}
               aria-label={edgeLabel(e)}
               aria-disabled={!live}
               onClick={() => { if (live) onMove(e); }}
               onKeyDown={(ev) => {
                 if (ev.key !== "Enter" && ev.key !== " ") return;
                 ev.preventDefault();
                 if (live) onMove(e);
               }}>
              <Line e={e} className="game-guide" />
              <Line e={e} className="game-hit" inset={4} />
            </g>
          );
        })}

        {Array.from({ length: N * N }, (_, i) => {
          const r = Math.floor(i / N), c = i % N, on = touched(r, c);
          return (
            <g key={`s${i}`} aria-hidden
               className={`game-star ${(r + c) % 2 ? "game-star--cyan" : ""} ${on ? "game-star--on" : ""}`}
               style={{ animationDelay: `${(i * 0.73) % 6}s` }}>
              <circle className="game-star-glow" cx={at(c)} cy={at(r)} r={3.8} />
              <circle className="game-star-core" cx={at(c)} cy={at(r)} r={on ? 1.45 : 1.15} />
            </g>
          );
        })}
      </svg>
    </div>
  );
}

function Line({ e, className, inset = 0, pathLength }: {
  e: number; className: string; inset?: number; pathLength?: number;
}) {
  const [a, b] = ends(e);
  const h = a.r === b.r;
  const x1 = at(a.c) + (h ? inset : 0), y1 = at(a.r) + (h ? 0 : inset);
  const x2 = at(b.c) - (h ? inset : 0), y2 = at(b.r) - (h ? 0 : inset);
  return <line x1={x1} y1={y1} x2={x2} y2={y2} className={className} pathLength={pathLength} />;
}

/** a concave four point star, the same family as the stars it sits between */
function sparkle(cx: number, cy: number, s: number): string {
  return `M${cx} ${cy - s}Q${cx} ${cy} ${cx + s} ${cy}Q${cx} ${cy} ${cx} ${cy + s}` +
         `Q${cx} ${cy} ${cx - s} ${cy}Q${cx} ${cy} ${cx} ${cy - s}Z`;
}
