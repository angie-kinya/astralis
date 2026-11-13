"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { useMe } from "@/components/AppGate";
import ActiveGames from "@/components/game/ActiveGames";
import ConstellationsStart from "@/components/game/ConstellationsStart";
import { ConstellationGlyph, SudokuGlyph, WordsGlyph } from "@/components/game/Glyphs";
import Leaderboard from "@/components/game/Leaderboard";
import SudokuStart from "@/components/sudoku/SudokuStart";
import WordsStart from "@/components/words/WordsStart";
import { type Game, activeGames, userMessage } from "@/lib/api";
import { useEvent } from "@/lib/events";

/**
 * every game in one place: what you can pick back up, a way into each game,
 * and the week's board. games with a person still start from their chat in
 * the circle, so nobody is invited by a stranger from here
 */
export default function PlayPage() {
  const { me } = useMe();
  const [games, setGames] = useState<Game[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [boardKey, setBoardKey] = useState(0);

  const load = useCallback(async () => {
    try { setGames((await activeGames()).games); }
    catch (e) { setError(userMessage(e)); }
  }, []);
  useEffect(() => { if (me.games) void load(); }, [me.games, load]);

  useEvent<{ game: Game }>("game.update", ({ game }) => {
    if (!game) return;
    setGames((gs) => {
      const rest = (gs ?? []).filter((g) => g.id !== game.id);
      return game.status === "active" ? [game, ...rest] : rest;
    });
    if (game.status === "finished" && game.mode === "peer") setBoardKey((k) => k + 1);
  });

  if (!me.games) {
    return (
      <div className="grid h-full place-items-center px-6 text-center">
        <p className="font-mono text-[.75rem] text-ink-3">
          nothing to play here yet. <Link href="/chat" className="text-ink-2 underline underline-offset-2">back to astralis</Link>
        </p>
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-3xl px-4 py-6 sm:py-8">
        <p className="label">play</p>
        <h1 className="mt-2 font-sans text-[1.375rem] font-semibold tracking-[-.02em] text-ink">A few quiet minutes</h1>
        <p className="mt-1.5 max-w-measure font-sans text-[.875rem] leading-relaxed text-ink-2">
          Small games to rest your head between other things. No clocks on screen, no streaks, nothing to keep up.
        </p>

        {games && games.length > 0 && (
          <section aria-labelledby="active-title" className="mt-7">
            <h2 id="active-title" className="label mb-2">pick up where you left off</h2>
            <ActiveGames games={games} />
          </section>
        )}
        {error && <p role="alert" className="mt-4 font-mono text-[.6875rem] text-amber">{error}</p>}

        <div className="mt-7 grid gap-3 md:grid-cols-2">
          <GameCard id="constellations" title="constellations" accent="--violet-2" glyph={<ConstellationGlyph className="h-4 w-4" />}
                    blurb="Take turns joining stars. Close a square to keep it and draw again."
                    className="md:col-span-2">
            <ConstellationsStart />
          </GameCard>
          <GameCard id="words" title="words" accent="--g-teal" glyph={<WordsGlyph className="h-4 w-4" />}
                    blurb="Find the hidden word. Hints are free and never cost you anything.">
            <WordsStart />
          </GameCard>
          <GameCard id="sudoku" title="sudoku" accent="--g-sky" glyph={<SudokuGlyph className="h-4 w-4" />}
                    blurb="Fill the grid so every row, column and box holds each number once.">
            <SudokuStart />
          </GameCard>
        </div>

        <section aria-label="this week" className="card mt-7 p-5">
          <Leaderboard refreshKey={boardKey} />
        </section>

        <section aria-labelledby="points-title" className="mt-4 px-1 pb-4">
          <h2 id="points-title" className="label">how points work</h2>
          <p className="mt-1.5 max-w-measure font-mono text-[.6875rem] leading-relaxed text-ink-3">
            with someone in the circle: finishing earns 1, a shared sky (a draw) 2 each, a win 3. with astralis:
            finishing 1, a win 2. words: finishing 1, solving 2, and 1 more for today&apos;s puzzle. sudoku: solving
            earns its level (mini 1 up to hard 4), and 1 more for today&apos;s. practice (astralis and extra puzzles)
            adds up to 10 points a day. playing both colours on your own is just practice and earns none. the board
            starts fresh every Monday, and there are no streaks.
          </p>
        </section>
      </div>
    </div>
  );
}

// each game keeps one quiet accent, a tinted glyph and a top edge, and nothing louder
function GameCard({ id, title, accent, glyph, blurb, className = "", children }: {
  id: string; title: string; accent: string; glyph: React.ReactNode; blurb: string; className?: string;
  children: React.ReactNode;
}) {
  return (
    <section aria-labelledby={`${id}-title`} className={`game-pane p-4 sm:p-5 ${className}`}
             style={{ borderTopColor: `rgb(var(${accent}) / .55)` }}>
      <h2 id={`${id}-title`} className="flex items-center gap-2 font-mono text-[.75rem] text-ink">
        <span style={{ color: `rgb(var(${accent}))` }}>{glyph}</span>
        {title}
      </h2>
      <p className="mt-1 font-sans text-[.8125rem] leading-relaxed text-ink-3">{blurb}</p>
      <div className="mt-3.5">{children}</div>
    </section>
  );
}
