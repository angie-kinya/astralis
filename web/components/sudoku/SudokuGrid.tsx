"use client";

import { useEffect, useRef } from "react";

import type { Notes } from "@/components/sudoku/logic";

/**
 * the board. every cell is a button with a full label, and the grid uses a
 * roving tabindex: tab lands on the selected cell, arrows move within. the
 * parent owns the keys (so digits typed while the pad has focus still land)
 * and tells the grid when focus should follow the selection.
 *
 * shading, softest first: the selected cell's row, column and box; cells
 * holding the same digit; repeats (a warm tint, never an alarm); the
 * selection itself
 */
export default function SudokuGrid({
  size, boxRows, boxCols, cells, given, notes, selected, conflicts, readOnly, focusTick, onSelect,
}: {
  size: number; boxRows: number; boxCols: number;
  cells: string[]; given: (i: number) => boolean; notes: Notes;
  selected: number; conflicts: Set<number>; readOnly: boolean;
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
         className="mx-auto grid aspect-square w-full max-w-[28rem] overflow-hidden rounded-xl border-2 border-line-2
                    bg-surface/70 [touch-action:manipulation]"
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
            const mask = notes[i] ?? 0;

            const bg = sel ? "bg-violet/20"
              : bad ? "bg-amber/[.13]"
              : same ? "bg-violet/[.11]"
              : near ? "bg-raised/70" : "";
            const ink = bad && !g ? "text-amber" : g ? "text-ink font-medium" : "text-violet-2";

            return (
                <button
                  key={c}
                  ref={(el) => { refs.current[i] = el; }}
                  type="button"
                  tabIndex={i === selected ? 0 : -1}
                  onClick={() => onSelect(i)}
                  aria-label={label(r, c, d, g, mask, bad, size) + (sel ? ", selected" : "")}
                  className={`relative flex items-center justify-center border-line font-mono outline-none
                              transition-colors duration-150 focus-visible:z-10 focus-visible:ring-2
                              focus-visible:ring-inset focus-visible:ring-violet focus-visible:ring-offset-0
                              ${bg} ${ink} ${text} ${sel ? "shadow-[inset_0_0_0_2px_rgb(var(--violet)/.7)]" : ""}`}
                  style={{
                    borderRightWidth: c === size - 1 ? 0 : (c + 1) % boxCols === 0 ? 2 : 1,
                    borderBottomWidth: r === size - 1 ? 0 : (r + 1) % boxRows === 0 ? 2 : 1,
                    borderRightColor: (c + 1) % boxCols === 0 ? "rgb(var(--line-2))" : undefined,
                    borderBottomColor: (r + 1) % boxRows === 0 ? "rgb(var(--line-2))" : undefined,
                  }}>
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

function label(r: number, c: number, d: string, given: boolean, mask: number, bad: boolean, size: number): string {
  const where = `row ${r + 1}, column ${c + 1}`;
  if (d !== "0") return `${where}, ${given ? "given" : "yours"} ${d}${bad ? ", repeats nearby" : ""}`;
  const marks: number[] = [];
  for (let k = 1; k <= size; k++) if (mask & (1 << k)) marks.push(k);
  return `${where}, empty${marks.length ? `, notes ${marks.join(" ")}` : ""}`;
}
