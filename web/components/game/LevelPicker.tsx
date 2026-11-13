"use client";

import type { Catalog } from "@/lib/api";
import { LEVEL_NAMES } from "@/components/game/rules";

export type Level = Catalog["constellations"]["levels"][number];

// what the engine ships, so the picker still reads if the catalog is slow
const FALLBACK: Level[] = [
  { level: 1, name: LEVEL_NAMES[1]!, rows: 3, cols: 3, boxes: 4 },
  { level: 2, name: LEVEL_NAMES[2]!, rows: 4, cols: 4, boxes: 9 },
  { level: 3, name: LEVEL_NAMES[3]!, rows: 5, cols: 5, boxes: 16 },
  { level: 4, name: LEVEL_NAMES[4]!, rows: 6, cols: 6, boxes: 0 },
  { level: 5, name: LEVEL_NAMES[5]!, rows: 7, cols: 7, boxes: 36 },
];

/**
 * the five skies. a locked level is shown, quietly, with what opens it:
 * finishing (not winning) the one before. nothing here counts down or nags
 */
export default function LevelPicker({ levels, unlocked, value, onChange, cap }: {
  levels: Level[] | undefined;
  /** highest playable level */
  unlocked: number;
  value: number;
  onChange: (level: number) => void;
  /** only show this many (the circle picker shows what you have open, plus the next) */
  cap?: number;
}) {
  const all = levels && levels.length ? levels : FALLBACK;
  const shown = cap ? all.filter((l) => l.level <= cap) : all;
  const next = all.find((l) => l.level === unlocked + 1);
  const before = all.find((l) => l.level === unlocked);

  return (
    <div>
      <div role="radiogroup" aria-label="sky" className="grid grid-cols-2 gap-1.5 min-[420px]:grid-cols-3">
        {shown.map((l) => {
          const locked = l.level > unlocked;
          const on = l.level === value;
          return (
            <button key={l.level} type="button" role="radio" aria-checked={on} aria-disabled={locked}
                    onClick={() => { if (!locked) onChange(l.level); }}
                    title={locked && before && l.level === unlocked + 1 ? `finish ${before.name} to open ${l.name}` : undefined}
                    className={`min-h-[2.75rem] rounded-lg border px-2.5 py-1.5 text-left transition-colors duration-150
                                ${on ? "border-violet/60 bg-violet/10 text-ink"
                                  : locked ? "cursor-default border-dashed border-line text-ink-3"
                                  : "border-line text-ink-2 hover:border-line-2 hover:bg-raised/60"}`}>
              <span className="block font-mono text-[.6875rem]">{l.name}</span>
              <span className="block font-mono text-[.5625rem] text-ink-3">
                {locked ? "not yet open" : `${l.rows}x${l.cols} stars${l.boxes ? ` · ${l.boxes} squares` : ""}`}
              </span>
            </button>
          );
        })}
      </div>
      {next && before && (
        <p className="mt-2 font-mono text-[.5625rem] leading-relaxed text-ink-3">
          finish {before.name} to open {next.name}. any finish counts, with astralis or on your own.
        </p>
      )}
    </div>
  );
}

export function levelName(levels: Level[] | undefined, n: number): string {
  return levels?.find((l) => l.level === n)?.name ?? LEVEL_NAMES[n] ?? `level ${n}`;
}
