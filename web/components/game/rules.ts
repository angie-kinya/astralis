import type { Game, Player } from "@/lib/api";

/**
 * the board's geometry and just enough of the rules to draw a move before the
 * server agrees. the server (internal/game) is the only referee: anything
 * computed here is a guess that the next response overwrites
 */

export const N = 4;            // stars per side
export const EDGES = 24;
export const BOXES = 9;

export type Star = { r: number; c: number };

/** 0..11 join (r,c)-(r,c+1); 12..23 join (r,c)-(r+1,c) */
export function ends(e: number): [Star, Star] {
  if (e < 12) {
    const r = Math.floor(e / 3), c = e % 3;
    return [{ r, c }, { r, c: c + 1 }];
  }
  const k = e - 12, r = Math.floor(k / 4), c = k % 4;
  return [{ r, c }, { r: r + 1, c }];
}

export function boxEdges(b: number): number[] {
  const r = Math.floor(b / 3), c = b % 3;
  return [r * 3 + c, (r + 1) * 3 + c, 12 + r * 4 + c, 12 + r * 4 + c + 1];
}

export function boxesOf(e: number): number[] {
  const [a] = ends(e);
  const out: number[] = [];
  if (e < 12) {
    if (a.r > 0) out.push((a.r - 1) * 3 + a.c);
    if (a.r < N - 1) out.push(a.r * 3 + a.c);
  } else {
    if (a.c > 0) out.push(a.r * 3 + a.c - 1);
    if (a.c < N - 1) out.push(a.r * 3 + a.c);
  }
  return out;
}

/** a row-by-row reading order (a row's horizontals, then the verticals below it), so tab follows the eye */
export const READING_ORDER: number[] = (() => {
  const out: number[] = [];
  for (let r = 0; r < N; r++) {
    for (let c = 0; c < N - 1; c++) out.push(r * 3 + c);
    if (r < N - 1) for (let c = 0; c < N; c++) out.push(12 + r * 4 + c);
  }
  return out;
})();

export function edgeLabel(e: number): string {
  const [a, b] = ends(e);
  return `join star row ${a.r + 1} column ${a.c + 1} to row ${b.r + 1} column ${b.c + 1}`;
}

/** the optimistic move: same rules as the engine, version untouched so the server's reply always wins */
export function applyLocal(g: Game, e: number): Game {
  const p = g.mode === "solo" ? g.toMove : g.me;
  const drawn = g.drawn.slice();
  drawn[e] = true;
  const owner = g.owner.slice();
  let claimed = false;
  for (const b of boxesOf(e)) {
    if (owner[b] === 0 && boxEdges(b).every((x) => drawn[x])) { owner[b] = p; claimed = true; }
  }
  const toMove = (claimed ? p : 3 - p) as 1 | 2;
  const mine = owner.filter((o) => o === g.me).length;
  const theirs = owner.filter((o) => o !== 0 && o !== g.me).length;
  const over = drawn.every(Boolean);
  return {
    ...g, drawn, owner, toMove, lastEdge: e, botEdges: [],
    drawnBy: g.drawnBy ? Object.assign(g.drawnBy.slice(), { [e]: p }) : undefined,
    myTurn: !over && (g.mode === "solo" || toMove === g.me),
    score: { me: mine, them: theirs },
    status: over ? "finished" : g.status,
    result: over && g.mode !== "solo" ? (mine > theirs ? "won" : "lost") : null,
  };
}

export function count(owner: Player[], p: Player): number {
  return owner.filter((o) => o === p).length;
}
