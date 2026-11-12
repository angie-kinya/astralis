"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { ConstellationGlyph } from "@/components/game/InviteCard";
import Leaderboard from "@/components/game/Leaderboard";
import { type Game, activeGames, createGame, userMessage } from "@/lib/api";
import { useEvent } from "@/lib/events";

/**
 * the quiet way in: games you can pick back up, a game with astralis or on
 * your own (neither needs the circle), and the week's board. games with a
 * person start from their thread, never from here, so nobody is cold-invited
 */
export default function ConstellationsPanel({ className = "" }: { className?: string }) {
  const router = useRouter();
  const [games, setGames] = useState<Game[] | null>(null);
  const [starting, setStarting] = useState<"astralis" | "solo" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [boardKey, setBoardKey] = useState(0);

  const load = useCallback(async () => {
    try { setGames((await activeGames()).games); }
    catch (e) { setError(userMessage(e)); }
  }, []);
  useEffect(() => { void load(); }, [load]);

  useEvent<{ game: Game }>("game.update", ({ game }) => {
    if (!game) return;
    setGames((gs) => {
      const rest = (gs ?? []).filter((g) => g.id !== game.id);
      return game.status === "active" ? [game, ...rest] : rest;
    });
    if (game.status === "finished" && game.mode === "peer") setBoardKey((k) => k + 1);
  });

  async function start(mode: "astralis" | "solo") {
    setStarting(mode); setError(null);
    try {
      const { game } = await createGame(mode);
      router.push(`/circle/play/${game.id}`);
    } catch (e) { setError(userMessage(e)); setStarting(null); }
  }

  return (
    <section aria-labelledby="constellations-title" className={className}>
      <h2 id="constellations-title" className="flex items-center gap-2 font-mono text-[.6875rem] text-ink-2">
        <ConstellationGlyph className="h-4 w-4 text-violet-2" />
        constellations
      </h2>
      <p className="mt-1.5 font-sans text-[.8125rem] leading-relaxed text-ink-3">
        Take turns joining stars. Close a square to keep it and draw again.
      </p>

      <div className="mt-3 flex flex-wrap gap-2">
        <button className="btn-quiet !px-3 !py-1.5" disabled={!!starting} onClick={() => void start("astralis")}>
          {starting === "astralis" ? "Starting…" : "Play Astralis"}
        </button>
        <button className="btn-quiet !px-3 !py-1.5" disabled={!!starting} onClick={() => void start("solo")}>
          {starting === "solo" ? "Starting…" : "Play solo"}
        </button>
      </div>
      <p className="mt-1.5 font-mono text-[.5625rem] text-ink-3">
        to play someone, open their thread and invite them from the menu.
      </p>

      {games && games.length > 0 && (
        <ul className="mt-4 space-y-1">
          {games.map((g) => (
            <li key={g.id}>
              <Link href={`/circle/play/${g.id}`}
                    className="flex items-center gap-2.5 rounded-lg border border-line px-3 py-2
                               transition-colors duration-150 hover:border-line-2 hover:bg-raised/60">
                <span className={g.myTurn ? "dot-on" : "dot-off"} aria-hidden />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-mono text-[.75rem] text-ink">
                    {g.mode === "peer" ? `with ${g.peerHandle}` : g.mode === "astralis" ? "with astralis" : "solo"}
                  </span>
                  <span className="block font-mono text-[.5625rem] text-ink-3">
                    {g.mode === "solo"
                      ? `violet ${g.score.me} · cyan ${g.score.them}`
                      : `${g.myTurn ? "your move" : "their move"} · you ${g.score.me} · ${g.mode === "astralis" ? "astralis" : "them"} ${g.score.them}`}
                  </span>
                </span>
                <span className="font-mono text-[.5625rem] uppercase tracking-[.12em] text-violet-2">resume</span>
              </Link>
            </li>
          ))}
        </ul>
      )}

      {error && <p role="alert" className="mt-3 font-mono text-[.6875rem] text-amber">{error}</p>}

      <div className="hair my-4" />
      <Leaderboard refreshKey={boardKey} />
    </section>
  );
}
