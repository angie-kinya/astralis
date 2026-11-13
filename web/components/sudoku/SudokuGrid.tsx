"use client";

import { useEffect, useRef } from "react";

import type { Notes } from "@/components/sudoku/logic";

/**
 * the board. every cell is a button with a full label, and the grid uses a
 * roving tabindex: tab lands on the selected cell, arrows move within. the
 * parent owns the keys (so digits typed while the pad has focus still land)
 * and tells the grid when focus should follow the selection.
 *
 * shading, softest first: a faint sky wash on the selected cell's row,
 * column and box; a faint teal on cells holding the same digit; rose on a
 * wrong digit or a repeat; sky on the selection itself. givens are ink, the
 * player's digits violet. rose is never the only cue: those cells also carry
 * a corner dot and say "doesn't fit" or "repeats nearby" in their label
 */
export default function SudokuGrid({
  size, boxRows, boxCols, cells, given, notes, selected, conflicts, wrong, readOnly, glow, focusTick, onSelect,
}: {
  size: number; boxRows: number; boxCols: number;
  cells: string[]; given: (i: number) => boolean; notes: Notes;
  selected: number; conflicts: Set<number>;
  /** cells the server judged wrong, already trimmed to their current value */
  wrong: Set<number>;
  readOnly: boolean;
  /** solved: the grid settles into a soft teal glow */
  glow: boolean;
  /** bumps when the keyboard moved the selection and focus should follow */
  focusTick: number;
  onSelect: (i: number) => void;
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  useEffect(() => { if (focusTick) refs.current[selected]?.focus(); }, [focusTick, selected]);

  const sr = Math.floor(selected / size), sc = selected % size;
  const sb = Math.floor(sr / boxRows) * boxRows * 100 + Math.floor(sc / boxCols);
  const sd = cells[selected] ?? "0";
  const text = size === 9 ? "text-[clamp(1rem,4.8vw,1.375rem)]" : "text-[clamp(1.125rem,6vw,1.625rem)]";
  const noteCols = size === 9 ? 3 : boxCols;

  return (
    <div role="group" aria-label={`sudoku, ${size} by ${size}${readOnly ? ", solved" : ""}`}
         className={`mx-auto grid aspect-square w-full max-w-[28rem] overflow-hidden rounded-xl border-2
                     bg-surface/70 [touch-action:manipulation] ${glow ? "g-glow" : "border-line-2"}`}
         style={{ gridTemplateRows: `repeat(${size}, minmax(0, 1fr))` }}>
      {Array.from({ length: size }, (_, r) => (
        <div key={r} className="grid" style={{ gridTemplateColumns: `repeat(${size}, minmax(0, 1fr))` }}>
          {Array.from({ length: size }, (_, c) => {
            const i = r * size + c;
            const d = cells[i] ?? "0";
            const g = given(i);
            const sel = i === selected && !readOnly;
            const box = Math.floor(r / boxRows) * boxRows * 100 + Math.floor(c / boxCols);
            const near = !readOnly && (r === sr || c === sc || box === sb);
            const same = !readOnly && sd !== "0" && d === sd;
            const bad = conflicts.has(i);
            const off = wrong.has(i);
            // a given is never the mistake, so it keeps its ink and only shares the tint
            const marked = (bad || off) && !g;
            const mask = notes[i] ?? 0;

            const bg = sel ? "bg-g-sky/20"
              : bad || off ? "bg-g-rose/[.12]"
              : same ? "bg-g-teal/[.11]"
              : near ? "bg-g-sky/[.06]" : "";
            const ink = marked ? "text-g-rose font-medium" : g ? "text-ink font-medium" : "text-violet-2";

            return (
                <button
                  key={c}
                  ref={(el) => { refs.current[i] = el; }}
                  type="button"
                  tabIndex={i === selected ? 0 : -1}
                  onClick={() => onSelect(i)}
                  aria-label={label(r, c, d, g, mask, bad, off, size) + (sel ? ", selected" : "")}
                  className={`relative flex items-center justify-center border-line font-mono outline-none
                              transition-colors duration-150 focus-visible:z-10 focus-visible:ring-2
                              focus-visible:ring-inset focus-visible:ring-g-sky focus-visible:ring-offset-0
                              ${bg} ${ink} ${text} ${sel ? "shadow-[inset_0_0_0_2px_rgb(var(--g-sky)/.75)]" : ""}`}
                  style={{
                    borderRightWidth: c === size - 1 ? 0 : (c + 1) % boxCols === 0 ? 2 : 1,
                    borderBottomWidth: r === size - 1 ? 0 : (r + 1) % boxRows === 0 ? 2 : 1,
                    borderRightColor: (c + 1) % boxCols === 0 ? "rgb(var(--line-2))" : undefined,
                    borderBottomColor: (r + 1) % boxRows === 0 ? "rgb(var(--line-2))" : undefined,
                  }}>
                  {marked && d !== "0" && (
                    <span aria-hidden className="absolute right-[9%] top-[9%] h-[clamp(4px,1.1vw,6px)] w-[clamp(4px,1.1vw,6px)]
                                                 rounded-full bg-g-rose" />
                  )}
                  {d !== "0" ? d : mask ? (
                    <span aria-hidden className="grid h-full w-full p-[8%] font-mono text-[clamp(.4375rem,1.9vw,.625rem)]
                                                 leading-none text-ink-3"
                          style={{ gridTemplateColumns: `repeat(${noteCols}, minmax(0, 1fr))` }}>
                      {Array.from({ length: size }, (_, k) => (
                        <span key={k} className="flex items-center justify-center">
                          {mask & (1 << (k + 1)) ? k + 1 : ""}
                        </span>
                      ))}
                    </span>
                  ) : null}
                </button>
            );
          })}
        </div>
      ))}
    </div>
  );
}

function label(r: number, c: number, d: string, given: boolean, mask: number, bad: boolean, off: boolean, size: number): string {
  const where = `row ${r + 1}, column ${c + 1}`;
  if (d !== "0") {
    return `${where}, ${given ? "given" : "yours"} ${d}${off ? ", doesn't fit" : ""}${bad ? ", repeats nearby" : ""}`;
  }
  const marks: number[] = [];
  for (let k = 1; k <= size; k++) if (mask & (1 << k)) marks.push(k);
  return `${where}, empty${marks.length ? `, notes ${marks.join(" ")}` : ""}`;
}
