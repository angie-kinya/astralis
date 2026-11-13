"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import NumberPad from "@/components/sudoku/NumberPad";
import SudokuGrid from "@/components/sudoku/SudokuGrid";
import {
  LEVEL_NAME, type Notes, cellsOf, conflicts, dropNotes, isGiven, loadNotes, norm, peers, saveNotes,
} from "@/components/sudoku/logic";
import { ApiError, type Game, type SudokuView, createGame, getGame, playMove, userMessage } from "@/lib/api";

// ms after the last input before the grid is saved. short, so a wrong digit
// turns red almost as it lands; a quick run of digits still rides one save
const QUIET = 150;
const DEPTH = 200;  // undo steps kept

type Snap = { cells: string[]; notes: Notes };

/**
 * one sudoku, solo and untimed.
 *
 *  - this tab's grid is the truth while it is open. edits are saved as the
 *    whole grid, 150 ms after the last one, or at once when the grid is full
 *    (the server is the only one who can say solved). one save runs at a
 *    time; edits made meanwhile ride the next one.
 *  - a newer game from outside (another tab, via the page) is taken only when
 *    nothing local is waiting to be saved; a 409 refetches and takes the
 *    server's grid.
 *  - a digit the server says is wrong, and any repeat, is marked rose with a
 *    corner dot. the server's marks only hold for the value it saw: a cell
 *    changed here since the last save drops its mark until the next one.
 *    there is no mistake count and no clock.
 *  - pencil notes never leave the browser: localStorage per game id
 */
export default function SudokuGame({ game, onGame }: { game: Game; onGame: (g: Game) => void }) {
  const router = useRouter();
  const view = game.view as SudokuView;
  const { size, boxRows, boxCols } = view;
  const solved = view.solved || game.status !== "active";

  const [cells, setCells] = useState<string[]>(() => cellsOf(view));
  const [notes, setNotes] = useState<Notes>({});
  const [selected, setSelected] = useState(() => Math.max(0, cellsOf(view).indexOf("0")));
  const [focusTick, setFocusTick] = useState(0);
  const [notesMode, setNotesMode] = useState(false);
  const [history, setHistory] = useState<Snap[]>([]);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [again, setAgain] = useState(false);

  // the newest game seen, whether from props or from a save that has not come back down yet
  const gameRef = useRef(game);
  if (game.id !== gameRef.current.id || game.version >= gameRef.current.version) gameRef.current = game;
  const cellsRef = useRef(cells);
  cellsRef.current = cells;
  const sent = useRef(cellsOf(view).join(""));
  const timer = useRef<number | undefined>(undefined);
  const inflight = useRef(false);
  const queued = useRef(false);

  // a different puzzle: start clean, with its notes from this browser
  useEffect(() => {
    const c = cellsOf(game.view as SudokuView);
    setCells(c); cellsRef.current = c; sent.current = c.join("");
    setNotes(loadNotes(game.id));
    setHistory([]); setNotice(null); setError(null);
    setSelected(Math.max(0, c.indexOf("0")));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);

  // a newer grid from elsewhere, taken only when nothing here is waiting
  useEffect(() => {
    const server = cellsOf(view).join("");
    if (server === sent.current || inflight.current) return;
    if (cellsRef.current.join("") !== sent.current) return;
    sent.current = server;
    setCells(server.split(""));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.version]);

  useEffect(() => { if (solved) dropNotes(game.id); }, [solved, game.id]);

  const flush = useCallback(async () => {
    window.clearTimeout(timer.current);
    if (inflight.current) { queued.current = true; return; }
    const grid = cellsRef.current.join("");
    const g = gameRef.current;
    if (grid === sent.current || g.status !== "active") return;
    inflight.current = true; setSaving(true); setError(null);
    try {
      const { game: next } = await playMove(g.id, { grid }, g.version);
      sent.current = grid;
      gameRef.current = next;
      onGame(next);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        try {
          const { game: fresh } = await getGame(g.id);
          const server = cellsOf(fresh.view as SudokuView);
          sent.current = server.join(""); cellsRef.current = server;
          setCells(server);
          gameRef.current = fresh;
          onGame(fresh);
          setNotice("this puzzle changed in another tab. here it is as it stands.");
        } catch (err) { setError(userMessage(err)); }
      } else {
        setError(userMessage(e, "That did not save. It will try again with your next number."));
      }
    } finally {
      inflight.current = false; setSaving(false);
      if (queued.current) { queued.current = false; void flush(); }
    }
  }, [onGame]);

  // leaving mid-pause still saves what was typed; nothing to hand up by then
  useEffect(() => () => {
    window.clearTimeout(timer.current);
    const g = gameRef.current;
    const grid = cellsRef.current.join("");
    if (grid !== sent.current && g.status === "active" && !inflight.current) {
      void playMove(g.id, { grid }, g.version).catch(() => {});
    }
  }, []);

  function commit(nextCells: string[], nextNotes: Notes) {
    setHistory((h) => [...h.slice(-(DEPTH - 1)), { cells: cellsRef.current, notes }]);
    if (nextNotes !== notes) { setNotes(nextNotes); saveNotes(game.id, nextNotes); }
    if (nextCells !== cellsRef.current) {
      cellsRef.current = nextCells;
      setCells(nextCells);
      window.clearTimeout(timer.current);
      if (!nextCells.includes("0")) void flush();
      else timer.current = window.setTimeout(() => void flush(), QUIET);
    }
  }

  const given = useCallback((i: number) => isGiven(view, i), [view]);
  const canEdit = !solved && !given(selected);

  function digit(d: number) {
    if (!canEdit) return;
    const i = selected;
    if (notesMode) {
      if (cells[i] !== "0") return;
      const mask = (notes[i] ?? 0) ^ (1 << d);
      const n = { ...notes };
      if (mask) n[i] = mask; else delete n[i];
      commit(cells, n);
      return;
    }
    const ch = String(d);
    const next = cells.slice();
    next[i] = cells[i] === ch ? "0" : ch;   // the same digit again lifts it off
    // a placed digit is crossed out of its neighbours' notes
    let n = notes;
    if (next[i] !== "0") {
      for (const p of peers(i, size, boxRows, boxCols)) {
        if (n[p] && n[p]! & (1 << d)) {
          if (n === notes) n = { ...notes };
          const m = n[p]! & ~(1 << d);
          if (m) n[p] = m; else delete n[p];
        }
      }
    }
    commit(next, n);
  }

  function erase() {
    if (!canEdit) return;
    const i = selected;
    if (cells[i] !== "0") {
      const next = cells.slice(); next[i] = "0";
      commit(next, notes);
    } else if (notes[i]) {
      const n = { ...notes }; delete n[i];
      commit(cells, n);
    }
  }

  function undo() {
    const last = history[history.length - 1];
    if (!last || solved) return;
    setHistory((h) => h.slice(0, -1));
    setNotes(last.notes); saveNotes(game.id, last.notes);
    if (last.cells !== cellsRef.current) {
      cellsRef.current = last.cells;
      setCells(last.cells);
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => void flush(), QUIET);
    }
  }

  function move(dr: number, dc: number) {
    const r = Math.min(size - 1, Math.max(0, Math.floor(selected / size) + dr));
    const c = Math.min(size - 1, Math.max(0, (selected % size) + dc));
    setSelected(r * size + c);
    setFocusTick((t) => t + 1);
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.altKey) return;
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") { e.preventDefault(); undo(); return; }
    if (e.ctrlKey || e.metaKey) return;
    const arrows: Record<string, [number, number]> = {
      ArrowUp: [-1, 0], ArrowDown: [1, 0], ArrowLeft: [0, -1], ArrowRight: [0, 1],
    };
    const a = arrows[e.key];
    if (a && !solved) { e.preventDefault(); move(a[0], a[1]); return; }
    const n = Number(e.key);
    if (e.key.length === 1 && n >= 1 && n <= size) { e.preventDefault(); digit(n); return; }
    if (e.key === "Backspace" || e.key === "Delete" || e.key === "0" || e.key === ".") { e.preventDefault(); erase(); return; }
    if (e.key === "n" || e.key === "N") { e.preventDefault(); setNotesMode((m) => !m); }
  }

  const bad = useMemo(() => conflicts(cells, size, boxRows, boxCols), [cells, size, boxRows, boxCols]);
  // the server's wrong marks, kept only where the cell still holds what it judged
  const wrong = useMemo(() => {
    const w = new Set<number>();
    for (const i of view.wrong ?? []) {
      if (!isGiven(view, i) && cells[i] !== "0" && cells[i] === norm(view.entries[i])) w.add(i);
    }
    return w;
  }, [view, cells]);
  const placed = useMemo(() => {
    const p = Array<number>(size + 1).fill(0);
    cells.forEach((d) => { if (d !== "0") p[Number(d)]! += 1; });
    return p;
  }, [cells, size]);
  const filled = cells.filter((d) => d !== "0").length;

  async function another() {
    setAgain(true); setError(null);
    try {
      const { game: g } = await createGame("sudoku", "solo", { level: view.level });
      router.push(`/play/${g.id}`);
    } catch (e) {
      setError(userMessage(e));
      setAgain(false);
    }
  }

  return (
    <div onKeyDown={onKeyDown}>
      <header className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
        <span className="tag">{LEVEL_NAME[view.level] ?? `level ${view.level}`}</span>
        <span className="font-mono text-[.8125rem] text-ink">{size} by {size}</span>
        <span className="ml-auto font-mono text-[.625rem] text-ink-3">
          {game.options.day ? "today's puzzle" : "practice"}
        </span>
      </header>

      <section className="bubble mt-3 border-line bg-surface/80 !px-3 !py-4 sm:!px-5 sm:!py-5">
        <div data-sudoku-grid>
          <SudokuGrid size={size} boxRows={boxRows} boxCols={boxCols} cells={cells} given={given} notes={notes}
                      selected={selected} conflicts={bad} wrong={wrong} readOnly={solved} glow={view.solved}
                      focusTick={focusTick}
                      onSelect={(i) => setSelected(i)} />
        </div>

        <p role="status" aria-live="polite" className="mt-3 min-h-[1.25rem] text-center font-mono text-[.6875rem] text-ink-3">
          {solved ? "" : bad.size ? "a number repeats in a row, column or box. it is marked, take your time."
            : wrong.size ? (wrong.size === 1 ? "one number doesn't fit where it is. it is marked, take your time."
                                             : "a few numbers don't fit where they are. they are marked, take your time.")
            : filled === cells.length ? (saving ? "checking…" : "every cell is filled, but one is not quite right yet. look again, gently.")
            : notesMode ? "notes on: numbers go in small" : ""}
        </p>

        {!solved && (
          <div className="mt-1">
            <NumberPad size={size} placed={placed} notesMode={notesMode} canEdit={canEdit}
                       canUndo={history.length > 0} onDigit={digit} onErase={erase}
                       onNotes={() => setNotesMode((m) => !m)} onUndo={undo} />
            <p className="mt-3 hidden text-center font-mono text-[.625rem] text-ink-3 sm:block">
              arrows to move, digits to place, backspace to clear, n for notes
            </p>
          </div>
        )}
      </section>

      {solved && (
        <section className="card mt-5 animate-rise p-5 text-center" aria-live="polite">
          <p className="label">{view.solved ? "solved" : "set down"}</p>
          <p className="mt-2 font-sans text-[1.0625rem] font-medium tracking-[-.01em] text-ink">
            {view.solved ? "every row, column and box sits right" : "this one was set down. nothing is lost"}
          </p>
          <p className="mx-auto mt-1.5 max-w-measure text-[.8125rem] leading-relaxed text-ink-2">
            {view.solved ? "Take a breath. Nothing was timed and nothing was counted." : "You can start a fresh one whenever you like."}
          </p>
          <div className="mt-4 flex flex-wrap justify-center gap-2">
            <Link href="/play" className="btn-quiet">Back to play</Link>
            <button type="button" className="btn-primary" disabled={again} onClick={() => void another()}>Another one</button>
          </div>
        </section>
      )}

      {notice && <p role="status" className="mt-4 text-center font-mono text-[.6875rem] text-ink-3">{notice}</p>}
      {error && <p role="alert" className="mt-4 text-center font-mono text-[.75rem] text-amber">{error}</p>}
    </div>
  );
}
