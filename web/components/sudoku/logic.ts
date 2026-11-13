import type { SudokuView } from "@/lib/api";

/** one char per cell, "0" empty. '.' from the server is read as empty too */
export const norm = (c: string | undefined) => (c && c >= "1" && c <= "9" ? c : "0");

/** the grid as the player sees it: givens where given, their entries elsewhere */
export function cellsOf(v: SudokuView): string[] {
  const n = v.size * v.size;
  return Array.from({ length: n }, (_, i) => {
    const g = norm(v.givens[i]);
    return g !== "0" ? g : norm(v.entries[i]);
  });
}

export const isGiven = (v: SudokuView, i: number) => norm(v.givens[i]) !== "0";

/** every cell sharing a row, column or box with i, excluding i */
export function peers(i: number, size: number, br: number, bc: number): number[] {
  const r = Math.floor(i / size), c = i % size;
  const r0 = r - (r % br), c0 = c - (c % bc);
  const out = new Set<number>();
  for (let k = 0; k < size; k++) { out.add(r * size + k); out.add(k * size + c); }
  for (let y = r0; y < r0 + br; y++) for (let x = c0; x < c0 + bc; x++) out.add(y * size + x);
  out.delete(i);
  return [...out];
}

/** cells whose digit repeats somewhere in their row, column or box */
export function conflicts(cells: string[], size: number, br: number, bc: number): Set<number> {
  const bad = new Set<number>();
  cells.forEach((d, i) => {
    if (d === "0") return;
    if (peers(i, size, br, bc).some((p) => cells[p] === d)) bad.add(i);
  });
  return bad;
}

// pencil notes live only in this browser, keyed by game: a bitmask of digits per cell
export type Notes = Record<number, number>;
const key = (id: string) => `astralis.sudoku.notes.${id}`;

export function loadNotes(id: string): Notes {
  try {
    const raw = window.localStorage.getItem(key(id));
    const v = raw ? (JSON.parse(raw) as unknown) : null;
    return v && typeof v === "object" ? (v as Notes) : {};
  } catch { return {}; }
}

export function saveNotes(id: string, notes: Notes) {
  try {
    if (Object.keys(notes).length) window.localStorage.setItem(key(id), JSON.stringify(notes));
    else window.localStorage.removeItem(key(id));
  } catch { /* notes are a convenience; the puzzle works without them */ }
}

export function dropNotes(id: string) {
  try { window.localStorage.removeItem(key(id)); } catch { /* nothing to do */ }
}

export const LEVEL_NAME: Record<number, string> = { 1: "mini", 2: "easy", 3: "medium", 4: "hard" };
