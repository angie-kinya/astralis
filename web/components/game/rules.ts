import type { ConstellationsView, Game, Player } from "@/lib/api";

/**
 * the board's geometry and just enough of the rules to draw a move before the
 * server agrees. the server (internal/game/constellations) is the only
 * referee: anything computed here is a guess that the next response overwrites.
 *
 * boards are rows x cols stars with an optional mask of active boxes. an
 * inactive box is empty sky: it can never be claimed, and an edge that borders
 * no active box is neither drawn nor playable
 */

export type Star = { r: number; c: number };

export type Geo = {
  rows: number;
  cols: number;
  /** horizontal edge count, the offset of the first vertical */
  H: number;
  edges: number;
  boxes: number;
  active: boolean[];
};

/** level names, used where the catalog is not loaded (invite cards, game lists) */
export const LEVEL_NAMES: Record<number, string> = {
  1: "first light", 2: "small dipper", 3: "lyra", 4: "orion", 5: "wide sky",
};
export const SUDOKU_LEVEL_NAMES: Record<number, string> = { 1: "mini", 2: "easy", 3: "medium", 4: "hard" };

export function view(g: Game): ConstellationsView {
  return g.view as ConstellationsView;
}

export function geometry(v: ConstellationsView): Geo {
  const H = v.rows * (v.cols - 1);
  const boxes = (v.rows - 1) * (v.cols - 1);
  // an absent or short mask means every box is part of the board
  const active = Array.from({ length: boxes }, (_, b) => v.active?.[b] ?? true);
  return { rows: v.rows, cols: v.cols, H, edges: H + (v.rows - 1) * v.cols, boxes, active };
}

export function ends(g: Geo, e: number): [Star, Star] {
  if (e < g.H) {
    const r = Math.floor(e / (g.cols - 1)), c = e % (g.cols - 1);
    return [{ r, c }, { r, c: c + 1 }];
  }
  const k = e - g.H, r = Math.floor(k / g.cols), c = k % g.cols;
  return [{ r, c }, { r: r + 1, c }];
}

export function boxEdges(g: Geo, b: number): number[] {
  const w = g.cols - 1, r = Math.floor(b / w), c = b % w;
  return [r * w + c, (r + 1) * w + c, g.H + r * g.cols + c, g.H + r * g.cols + c + 1];
}

/** the boxes either side of an edge, active or not */
export function sidesOf(g: Geo, e: number): number[] {
  const [a] = ends(g, e);
  const w = g.cols - 1;
  const out: number[] = [];
  if (e < g.H) {
    if (a.r > 0) out.push((a.r - 1) * w + a.c);
    if (a.r < g.rows - 1) out.push(a.r * w + a.c);
  } else {
    if (a.c > 0) out.push(a.r * w + a.c - 1);
    if (a.c < g.cols - 1) out.push(a.r * w + a.c);
  }
  return out;
}

/** the active boxes an edge can help close */
export function boxesOf(g: Geo, e: number): number[] {
  return sidesOf(g, e).filter((b) => g.active[b]);
}

export function playable(g: Geo, e: number): boolean {
  return boxesOf(g, e).length > 0;
}

/** a star is shown only when it is a corner of some active box */
export function starOn(g: Geo, r: number, c: number): boolean {
  const w = g.cols - 1;
  for (const [br, bc] of [[r - 1, c - 1], [r - 1, c], [r, c - 1], [r, c]] as const) {
    if (br >= 0 && bc >= 0 && br < g.rows - 1 && bc < w && g.active[br * w + bc]) return true;
  }
  return false;
}

/** a row-by-row reading order (a row's horizontals, then the verticals below it), so tab follows the eye */
export function readingOrder(g: Geo): number[] {
  const out: number[] = [];
  for (let r = 0; r < g.rows; r++) {
    for (let c = 0; c < g.cols - 1; c++) out.push(r * (g.cols - 1) + c);
    if (r < g.rows - 1) for (let c = 0; c < g.cols; c++) out.push(g.H + r * g.cols + c);
  }
  return out.filter((e) => playable(g, e));
}

export function edgeLabel(g: Geo, e: number): string {
  const [a, b] = ends(g, e);
  return `join star row ${a.r + 1} column ${a.c + 1} to row ${b.r + 1} column ${b.c + 1}`;
}

/** the optimistic move: same rules as the engine, version untouched so the server's reply always wins */
export function applyLocal(game: Game, e: number): Game {
  const v = view(game), g = geometry(v);
  const p: Player = game.mode === "solo" ? v.toMove : game.me;
  const drawn = v.drawn.slice();
  drawn[e] = true;
  const drawnBy = v.drawnBy.slice();
  drawnBy[e] = p;
  const owner = v.owner.slice();
  let claimed = false;
  for (const b of boxesOf(g, e)) {
    if (owner[b] === 0 && boxEdges(g, b).every((x) => drawn[x])) { owner[b] = p; claimed = true; }
  }
  const toMove = (claimed ? p : 3 - p) as 1 | 2;
  const over = drawn.every((d, x) => d || !playable(g, x));
  const a = game.mode === "solo" ? 1 : game.me;
  const mine = count(owner, a), theirs = count(owner, (3 - a) as Player);
  return {
    ...game,
    view: { ...v, drawn, drawnBy, owner, toMove, lastEdge: e },
    botEdges: [],
    myTurn: !over && (game.mode === "solo" || toMove === game.me),
    score: { me: mine, them: theirs },
    status: over ? "finished" : game.status,
    result: !over || game.mode === "solo" ? null : mine > theirs ? "won" : mine < theirs ? "lost" : "draw",
  };
}

export function count(owner: Player[], p: Player): number {
  return owner.filter((o) => o === p).length;
}
