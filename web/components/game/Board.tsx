"use client";

import { useEffect, useRef } from "react";

import type { Player } from "@/lib/api";
import { type Geo, edgeLabel, ends, readingOrder, starOn } from "@/components/game/rules";

/**
 * whose colour a player draws in: violet for you, amber for a person, orchid
 * (astralis's eye) for astralis, cyan for the second colour in solo. each is a
 * --p-* token in globals.css, >= 3:1 on the pane in both themes
 */
export type Tone = "you" | "them" | "bot" | "solo-b";

// stars S units apart with a margin of half a gap, so every edge's diamond hit
// area (star to star, box centre to box centre) tiles the board exactly
const S = 26, M = S / 2;
const at = (i: number) => M + i * S;

/**
 * the star field. purely a picture of the state it is handed plus one
 * callback; it never decides whether a move is allowed, it only refuses to
 * offer one while `live` is false.
 *
 * every playable undrawn edge is a real control: a focusable group whose hit
 * area is the diamond between its two stars and the centres of the boxes
 * beside it, so targets tile without overlap and stay at least 32px on a phone
 * (the width floor below is two rem per column).
 *
 * line weights are in screen pixels, not board units: --u (board units per
 * pixel) is measured, so a 3px line stays 3px on a 3x3 board and a 7x7 one
 */
export default function Board({
  geo, drawn, owner, drawnBy, lastEdge, run, tone, interactive, live, onMove,
}: {
  geo: Geo;
  drawn: boolean[];
  owner: Player[];
  drawnBy: Player[];
  lastEdge: number | null;
  /** the opponent's latest run of edges (astralis can draw several), haloed softly */
  run: number[];
  tone: (p: Player) => Tone;
  /** false once the game is over: undrawn edges leave the tab order */
  interactive: boolean;
  /** true only when a move from this tab would be legal right now */
  live: boolean;
  onMove: (edge: number) => void;
}) {
  const svg = useRef<SVGSVGElement>(null);
  const W = geo.cols * S, Hh = geo.rows * S;

  useEffect(() => {
    const el = svg.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(([entry]) => {
      const px = entry?.contentRect.width ?? 0;
      if (px > 0) el.style.setProperty("--u", String(W / px));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [W]);

  const order = readingOrder(geo);
  const touched = new Set<string>();
  drawn.forEach((d, e) => { if (d) ends(geo, e).forEach((s) => touched.add(`${s.r},${s.c}`)); });
  const last = lastEdge !== null && drawn[lastEdge] ? lastEdge : null;
  const lastTone = last !== null && drawnBy[last] ? tone(drawnBy[last]!) : null;

  return (
    <div className="game-board mx-auto"
         style={{
           aspectRatio: `${geo.cols} / ${geo.rows}`,
           width: `min(100%, max(${geo.cols * 2.25}rem, min(28rem, ${geo.cols * 6}rem, calc((100dvh - 19rem) * ${geo.cols / geo.rows}))))`,
         }}>
      <svg ref={svg} viewBox={`0 0 ${W} ${Hh}`} role="group" aria-label="constellations board"
           className="h-full w-full overflow-visible" data-live={live}>
        {/* claimed squares sit underneath everything: a wash of the owner's colour and their glyph in full */}
        {owner.map((p, b) => {
          if (!p || !geo.active[b]) return null;
          const w = geo.cols - 1, r = Math.floor(b / w), c = b % w;
          const x = at(c), y = at(r), cx = x + S / 2, cy = y + S / 2;
          const t = tone(p);
          return (
            <g key={`b${b}-${p}`} className={`game-box gp-${t}`}>
              <rect x={x + 2.4} y={y + 2.4} width={S - 4.8} height={S - 4.8} rx={2.4} />
              <Glyph tone={t} cx={cx} cy={cy} />
            </g>
          );
        })}

        {/* astralis's run this turn, then the very last line, which breathes until the next move */}
        {run.filter((e) => e !== last && drawn[e]).map((e) => (
          <Line key={`run-${e}`} geo={geo} e={e} className={`game-run gp-${drawnBy[e] ? tone(drawnBy[e]!) : "them"}`} />
        ))}
        {last !== null && lastTone && lastTone !== "you" && (
          <Line key={`pulse-${last}`} geo={geo} e={last} className={`game-pulse gp-${lastTone}`} />
        )}

        {order.map((e) => {
          if (drawn[e]) {
            const p = drawnBy[e] ?? 0;
            return (
              <Line key={`d${e}`} geo={geo} e={e} pathLength={1}
                    className={`game-line ${p ? `gp-${tone(p)}` : "game-line--old"}`} />
            );
          }
          return (
            <g key={`u${e}`} role="button" className="game-edge"
               tabIndex={interactive ? 0 : -1}
               aria-label={edgeLabel(geo, e)}
               aria-disabled={!live}
               onClick={() => { if (live) onMove(e); }}
               onKeyDown={(ev) => {
                 if (ev.key !== "Enter" && ev.key !== " ") return;
                 ev.preventDefault();
                 if (live) onMove(e);
               }}>
              <polygon className="game-hit" points={diamond(geo, e)} />
              <Line geo={geo} e={e} className="game-guide" />
            </g>
          );
        })}

        {Array.from({ length: geo.rows * geo.cols }, (_, i) => {
          const r = Math.floor(i / geo.cols), c = i % geo.cols;
          if (!starOn(geo, r, c)) return null; // empty sky
          const on = touched.has(`${r},${c}`);
          return (
            <g key={`s${i}`} aria-hidden
               className={`game-star ${on ? "game-star--on" : ""}`}
               style={{ animationDelay: `${(i * 0.73) % 6}s` }}>
              <circle className="game-star-glow" cx={at(c)} cy={at(r)} r={4.2} />
              <circle className="game-star-core" cx={at(c)} cy={at(r)} r={on ? 1.7 : 1.35} />
            </g>
          );
        })}
      </svg>
    </div>
  );
}

function Line({ geo, e, className, pathLength }: {
  geo: Geo; e: number; className: string; pathLength?: number;
}) {
  const [a, b] = ends(geo, e);
  return <line x1={at(a.c)} y1={at(a.r)} x2={at(b.c)} y2={at(b.r)} className={className} pathLength={pathLength} />;
}

/** star, box centre, star, box centre: the edge's share of the board */
function diamond(geo: Geo, e: number): string {
  const [a, b] = ends(geo, e);
  const x1 = at(a.c), y1 = at(a.r), x2 = at(b.c), y2 = at(b.r);
  const mx = (x1 + x2) / 2, my = (y1 + y2) / 2;
  return a.r === b.r
    ? `${x1},${y1} ${mx},${my - M} ${x2},${y2} ${mx},${my + M}`
    : `${x1},${y1} ${mx - M},${my} ${x2},${y2} ${mx + M},${my}`;
}

/** shape as well as colour, so the board reads without colour vision */
export function Glyph({ tone, cx, cy }: { tone: Tone; cx: number; cy: number }) {
  if (tone === "you") return <path className="game-box-fill" d={sparkle(cx, cy, 5.2)} />;
  if (tone === "bot") {
    // astralis's eye: an almond with its iris
    return (
      <g>
        <path className="game-box-ring" d={`M${cx - 5} ${cy}Q${cx} ${cy - 4.4} ${cx + 5} ${cy}Q${cx} ${cy + 4.4} ${cx - 5} ${cy}Z`} />
        <circle className="game-box-fill" cx={cx} cy={cy} r={1.5} />
      </g>
    );
  }
  if (tone === "solo-b") {
    return <rect className="game-box-ring" x={cx - 3.4} y={cy - 3.4} width={6.8} height={6.8} rx={.6}
                 transform={`rotate(45 ${cx} ${cy})`} />;
  }
  return <circle className="game-box-ring" cx={cx} cy={cy} r={3.8} />;
}

/** a concave four point star, the same family as the stars it sits between */
function sparkle(cx: number, cy: number, s: number): string {
  return `M${cx} ${cy - s}Q${cx} ${cy} ${cx + s} ${cy}Q${cx} ${cy} ${cx} ${cy + s}` +
         `Q${cx} ${cy} ${cx - s} ${cy}Q${cx} ${cy} ${cx} ${cy - s}Z`;
}

/** the legend's swatch: a short line in the player's colour with their glyph */
export function Swatch({ tone }: { tone: Tone }) {
  return (
    <svg viewBox="0 0 30 14" aria-hidden className={`game-swatch gp-${tone}`}>
      <line x1={2} y1={7} x2={16} y2={7} />
      <g transform="translate(23 7) scale(.62)"><Glyph tone={tone} cx={0} cy={0} /></g>
    </svg>
  );
}
