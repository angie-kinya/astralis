"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";

import GameView from "@/components/game/GameView";
import { KindGlyph } from "@/components/game/Glyphs";
import { IconBack } from "@/components/icons";
import SudokuGame from "@/components/sudoku/SudokuGame";
import WordsGame from "@/components/words/WordsGame";
import { ApiError, type Game, type GameKind, getGame, userMessage } from "@/lib/api";
import { useEvent, useEventsConnected } from "@/lib/events";

const KIND_NAME: Record<GameKind, string> = { constellations: "constellations", words: "words", sudoku: "sudoku" };

/**
 * one game of any kind: loads it, keeps it current, and hands it to the
 * board for its kind.
 *
 *  - game.update events are hints like every other event: taken only when
 *    newer than what is on screen, and a 60 s poll covers a dropped stream.
 *  - what a board hands back through onGame (an optimistic move, or the
 *    server's reply) is taken unless it is older, since an optimistic move
 *    shares its version with the state it was made on
 */
export default function PlayRoom({ gameId }: { gameId: string }) {
  const live = useEventsConnected();
  const [game, setGame] = useState<Game | null>(null);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const ref = useRef<Game | null>(null);

  const take = useCallback((next: Game, strict: boolean) => {
    const prev = ref.current;
    if (prev && prev.id === next.id && (strict ? next.version <= prev.version : next.version < prev.version)) return;
    ref.current = next;
    setGame(next);
  }, []);

  const load = useCallback(async () => {
    try { take((await getGame(gameId)).game, false); }
    catch (e) {
      if (e instanceof ApiError && e.status === 404) setMissing(true);
      else setError(userMessage(e));
    }
  }, [gameId, take]);

  useEffect(() => { void load(); }, [load]);

  useEvent<{ game: Game }>("game.update", ({ game: g }) => { if (g?.id === gameId) take(g, true); });

  const active = game?.status === "active";

  // only while the stream is down, and only while there is anything left to happen
  useEffect(() => {
    if (live || !active) return;
    const id = setInterval(() => void load(), 60_000);
    return () => clearInterval(id);
  }, [live, active, load]);

  // a tab coming back from the background catches up at once rather than at the next poll
  useEffect(() => {
    const onShow = () => { if (document.visibilityState === "visible") void load(); };
    document.addEventListener("visibilitychange", onShow);
    return () => document.removeEventListener("visibilitychange", onShow);
  }, [load]);

  const onGame = useCallback((g: Game) => take(g, false), [take]);

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-xl px-4 py-5 sm:py-7">
        <div className="mb-5 flex items-center justify-between">
          <Link href="/play"
                className="-ml-1 flex min-h-8 items-center gap-1 rounded-lg px-1 py-0.5 font-mono text-[.6875rem]
                           text-ink-3 transition-colors hover:text-ink">
            <IconBack className="h-3.5 w-3.5" />
            play
          </Link>
          {/* words and sudoku draw their own header (mode, level, daily or practice) */}
          {game?.kind === "constellations" && (
            <span className="label flex items-center gap-1.5">
              <KindGlyph kind={game.kind} className="h-3.5 w-3.5 text-violet-2" />
              {KIND_NAME[game.kind] ?? game.kind}
            </span>
          )}
        </div>

        {missing ? (
          <p className="font-mono text-[.75rem] leading-relaxed text-ink-3">
            this game is not here. it may belong to someone else, or the link is wrong.
          </p>
        ) : !game ? (
          error
            ? <p role="alert" className="font-mono text-[.75rem] text-amber">{error}</p>
            : <p className="font-mono text-[.75rem] text-ink-3">gathering the stars…</p>
        ) : game.kind === "words" ? (
          <WordsGame game={game} onGame={onGame} />
        ) : game.kind === "sudoku" ? (
          <SudokuGame game={game} onGame={onGame} />
        ) : (
          <GameView game={game} onGame={onGame} />
        )}
      </div>
    </div>
  );
}
