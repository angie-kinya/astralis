"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { type Catalog, catalog, createGame, userMessage } from "@/lib/api";

type Level = Catalog["sudoku"]["levels"][number];

const KEY = "astralis.sudoku.level";

/**
 * pick a size and depth, then today's puzzle or another one. mini (6 by 6)
 * is the default the first time: gentle, and comfortable on a phone
 */
export default function SudokuStart() {
  const router = useRouter();
  const [levels, setLevels] = useState<Level[] | null>(null);
  const [level, setLevel] = useState(1);
  const [busy, setBusy] = useState<"daily" | "practice" | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    catalog()
      .then((c) => {
        if (!live) return;
        const ls = c.sudoku.levels;
        let last = NaN;
        try { last = Number(window.localStorage.getItem(KEY)); } catch { /* fine without it */ }
        setLevels(ls);
        setLevel(ls.find((l) => l.level === last)?.level ?? ls[0]?.level ?? 1);
      })
      .catch((e) => { if (live) setError(userMessage(e)); });
    return () => { live = false; };
  }, []);

  async function start(daily: boolean) {
    setBusy(daily ? "daily" : "practice"); setError(null);
    try { window.localStorage.setItem(KEY, String(level)); } catch { /* fine without it */ }
    try {
      const { game } = await createGame("sudoku", "solo", daily ? { level, daily: true } : { level });
      router.push(`/play/${game.id}`);
    } catch (e) {
      setError(userMessage(e));
      setBusy(null);
    }
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (!levels?.length) return;
    const i = levels.findIndex((l) => l.level === level);
    const step = e.key === "ArrowRight" || e.key === "ArrowDown" ? 1 : e.key === "ArrowLeft" || e.key === "ArrowUp" ? -1 : 0;
    if (!step || i < 0) return;
    e.preventDefault();
    const next = levels[(i + step + levels.length) % levels.length]!;
    setLevel(next.level);
    (e.currentTarget.querySelector(`[data-level="${next.level}"]`) as HTMLElement | null)?.focus();
  }

  if (!levels) {
    return error
      ? <p role="alert" className="font-mono text-[.75rem] text-amber">{error}</p>
      : <p className="font-mono text-[.75rem] text-ink-3">ruling the grid…</p>;
  }

  return (
    <div className="space-y-5">
      <fieldset>
        <legend className="label mb-2">size and depth</legend>
        <div role="radiogroup" aria-label="size and depth" onKeyDown={onKeyDown}
             className="grid grid-cols-2 gap-2">
          {levels.map((l) => {
            const on = l.level === level;
            return (
              <button key={l.level} type="button" role="radio" aria-checked={on} data-level={l.level}
                      tabIndex={on ? 0 : -1} onClick={() => setLevel(l.level)}
                      className={`flex min-w-0 items-start gap-3 rounded-xl border px-3.5 py-3 text-left transition-colors duration-150
                                  ${on ? "border-violet/60 bg-violet/10" : "border-line bg-surface/60 hover:border-line-2 hover:bg-raised/60"}`}>
                <MiniGrid size={l.size} on={on} />
                <span className="min-w-0">
                  <span className={`block font-mono text-[.8125rem] ${on ? "text-ink" : "text-ink-2"}`}>{l.name}</span>
                  <span className="block font-mono text-[.625rem] text-ink-3">{l.size} by {l.size}</span>
                  {BLURB[l.level] && (
                    <span className="mt-1 block font-sans text-[.8125rem] leading-snug text-ink-2">{BLURB[l.level]}</span>
                  )}
                </span>
              </button>
            );
          })}
        </div>
      </fieldset>

      <div className="flex flex-wrap gap-2">
        <button type="button" className="btn-primary" disabled={!!busy} onClick={() => void start(true)}>
          {busy === "daily" ? "Opening…" : "Today's puzzle"}
        </button>
        <button type="button" className="btn-quiet" disabled={!!busy} onClick={() => void start(false)}>
          {busy === "practice" ? "Finding one…" : "Another one"}
        </button>
      </div>
      <p className="font-mono text-[.625rem] leading-relaxed text-ink-3">
        no clock, no mistake count. today&apos;s puzzle waits for you if you leave.
      </p>

      {error && <p role="alert" className="font-mono text-[.75rem] text-amber">{error}</p>}
    </div>
  );
}

/** a tiny drawing of the grid's boxes, so 6 by 6 and 9 by 9 read at a glance */
// a line per level, so the cards fill their column and say what each one feels
// like, the way the words topics do
const BLURB: Record<number, string> = {
  1: "a small grid for a short break",
  2: "every next number can be found",
  3: "a little more looking around",
  4: "slow, patient, deeply absorbing",
};

function MiniGrid({ size, on }: { size: number; on: boolean }) {
  const cols = size === 6 ? 2 : 3, rows = 3;
  return (
    <span aria-hidden className={`grid h-6 w-6 shrink-0 gap-[2px] ${on ? "text-violet-2" : "text-ink-3"}`}
          style={{ gridTemplateColumns: `repeat(${cols}, 1fr)`, gridTemplateRows: `repeat(${rows}, 1fr)` }}>
      {Array.from({ length: cols * rows }, (_, i) => (
        <span key={i} className="rounded-[2px] border border-current opacity-70" />
      ))}
    </span>
  );
}
