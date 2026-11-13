"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { type Leaderboard as Board, leaderboard, userMessage } from "@/lib/api";

/**
 * this week's board. only people who opted in are listed, but everyone sees
 * their own points. there are no streaks and no losses here on purpose:
 * finishing a game is worth something, and next monday everyone starts level
 */
export default function Leaderboard({ refreshKey = 0 }: { refreshKey?: number }) {
  const [board, setBoard] = useState<Board | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let gone = false;
    leaderboard()
      .then((b) => { if (!gone) setBoard(b); })
      .catch((e) => { if (!gone) setError(userMessage(e)); });
    return () => { gone = true; };
  }, [refreshKey]);

  return (
    <section aria-labelledby="board-title">
      <h3 id="board-title" className="label">this week</h3>
      {error && <p className="mt-2 font-mono text-[.6875rem] text-amber">{error}</p>}
      {!board && !error && <div className="skel mt-3 h-2 w-32" />}
      {board && (
        <>
          {board.top.length === 0 ? (
            <p className="mt-2 font-mono text-[.6875rem] leading-relaxed text-ink-3">
              nobody on the board yet this week.
            </p>
          ) : (
            <ol className="mt-2 space-y-0.5">
              {board.top.map((r, i) => (
                <li key={r.handle} aria-current={r.me ? "true" : undefined}
                    className={`flex items-center gap-2.5 rounded-lg px-2 py-1 font-mono text-[.75rem]
                                ${r.me ? "bg-violet/10 text-ink" : "text-ink-2"}`}>
                  <span className="w-5 shrink-0 text-right text-[.625rem] text-ink-3">{i + 1}</span>
                  <span className="min-w-0 flex-1 truncate">
                    {r.handle}
                    {r.me && <span className="ml-1.5 text-[.5625rem] uppercase tracking-[.12em] text-violet-2">you</span>}
                  </span>
                  <span className="shrink-0 tabular-nums">{r.points}</span>
                </li>
              ))}
            </ol>
          )}
          <p className="mt-3 font-mono text-[.6875rem] text-ink-2">
            your points this week: <b className="font-medium text-ink">{board.me.points}</b>
            {board.me.games > 0 && <span className="text-ink-3"> from {board.me.games} {board.me.games === 1 ? "game" : "games"}</span>}
          </p>
          {!board.me.optedIn && (
            <p className="mt-1 font-mono text-[.5625rem] leading-relaxed text-ink-3">
              only you can see these. you can appear on the board from{" "}
              <Link href="/settings" className="underline underline-offset-2 hover:text-ink-2">settings</Link>.
            </p>
          )}
          <p className="mt-1 font-mono text-[.5625rem] leading-relaxed text-ink-3">
            resets every Monday. every finished game counts; games with people count most.
          </p>
        </>
      )}
    </section>
  );
}
