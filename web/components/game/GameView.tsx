"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useLayoutEffect, useRef, useState } from "react";

import Butterfly from "@/components/Butterflies";
import Board, { Swatch, type Tone } from "@/components/game/Board";
import { levelName } from "@/components/game/LevelPicker";
import { applyLocal, boxEdges, boxesOf, count, geometry, playable, view } from "@/components/game/rules";
import { useCatalog, useProgress } from "@/components/game/useCatalog";
import { AstralisMark } from "@/components/Speaker";
import {
  ApiError, type Game, type Player, createGame, getGame, playMove, sendInvite, userMessage,
} from "@/lib/api";

const STEP = 350; // ms between astralis's lines when they are played back

const reducedMotion = () =>
  typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

// a layout effect, so astralis's reply is hidden before it is ever painted; plain effect on the server
const useBeforePaint = typeof window === "undefined" ? useEffect : useLayoutEffect;

/**
 * one game of constellations, in any of its three modes. the room above
 * (PlayRoom) owns loading, events and polling; this owns the board.
 *
 *  - the server is the referee. a move is drawn at once (applyLocal) and
 *    posted with the version it was drawn against; a 409 means the board
 *    moved underneath us, so the tab refetches and quietly takes the server's
 *    word. nothing is ever offered while it is not this tab's turn.
 *  - astralis answers inside the same request, so its lines arrive all at once
 *    in botEdges. they are hidden and revealed one by one, so its reply reads
 *    as moves rather than a jump (instantly under reduced motion), and stay
 *    haloed until you move so you can see everything it drew
 */
export default function GameView({ game, onGame }: { game: Game; onGame: (g: Game) => void }) {
  const router = useRouter();
  const cat = useCatalog();
  const levels = cat?.constellations.levels;
  const { unlocked, refresh: refreshProgress } = useProgress();
  const [hidden, setHidden] = useState<number[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [again, setAgain] = useState(false);

  const pendingRef = useRef(false);
  const timers = useRef<number[]>([]);
  const seen = useRef<{ version: number; lastEdge: number | null } | null>(null);
  // the last edge on screen before astralis's reply started, so the pulse has somewhere to sit
  const beforeBot = useRef<number | null>(null);

  const clearTimers = () => { timers.current.forEach((t) => window.clearTimeout(t)); timers.current = []; };
  useEffect(() => clearTimers, []);

  const v = view(game);
  const geo = geometry(v);

  useBeforePaint(() => {
    const prev = seen.current;
    seen.current = { version: game.version, lastEdge: v.lastEdge };
    // the optimistic copy shares its version with the state it came from: nothing to play back
    if (prev && prev.version === game.version) return;
    clearTimers();
    const queue = reducedMotion() ? [] : game.botEdges.filter((e) => v.drawn[e]);
    beforeBot.current = prev?.lastEdge ?? null;
    setHidden(queue);
    queue.forEach((_, i) => {
      timers.current.push(window.setTimeout(() => setHidden((h) => h.slice(1)), STEP * (i + 1)));
    });
  }, [game]);

  const active = game.status === "active";

  // a finish may open the next sky
  useEffect(() => { if (!active) refreshProgress(); }, [active, refreshProgress]);

  const canMove = active && game.myTurn && !pending && hidden.length === 0;

  async function move(e: number) {
    if (!canMove || v.drawn[e] || !playable(geo, e) || pendingRef.current) return;
    pendingRef.current = true; setPending(true); setError(null); setNotice(null);
    onGame(applyLocal(game, e));
    try {
      const { game: next } = await playMove(game.id, { edge: e }, game.version);
      onGame(next);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setNotice("the board moved on before that landed. here it is as it stands.");
      } else {
        setError(userMessage(err));
      }
      try { onGame((await getGame(game.id)).game); } catch { /* the room's poll catches up */ }
    } finally {
      pendingRef.current = false; setPending(false);
    }
  }

  async function playAgain(level: number) {
    setAgain(true); setError(null); setNotice(null);
    try {
      if (game.mode === "peer" && game.peerId) {
        await sendInvite(game.peerId, "constellations", { level });
        setNotice(`invited. it waits in your chat with ${game.peerHandle}.`);
      } else if (game.mode !== "peer") {
        const { game: g } = await createGame("constellations", game.mode, { level });
        router.push(`/play/${g.id}`);
        return;
      }
    } catch (e) {
      if (e instanceof ApiError && e.code === "invite_pending") {
        setNotice(`there is already an invite waiting with ${game.peerHandle}.`);
      } else setError(userMessage(e));
    }
    setAgain(false);
  }

  // what is on screen: the game minus any of astralis's lines still waiting to be revealed
  const hide = new Set(hidden);
  const drawnV = v.drawn.map((d, e) => d && !hide.has(e));
  const ownerV = v.owner.map((o, b) => (boxEdges(geo, b).every((e) => drawnV[e]) ? o : 0)) as Player[];
  const shown = game.botEdges.length - hidden.length;
  const lastV = hidden.length === 0 ? v.lastEdge
    : shown > 0 ? game.botEdges[shown - 1]! : beforeBot.current;
  const lastDrawer: Player = lastV === null ? 0 : v.drawnBy[lastV] ?? 0;
  const run = game.botEdges.slice(0, shown);

  const solo = game.mode === "solo";
  const a: Player = solo ? 1 : game.me;
  const b = (3 - a) as Player;
  const tone = (p: Player): Tone =>
    solo ? (p === 1 ? "you" : "solo-b") : p === game.me ? "you" : game.mode === "astralis" ? "bot" : "them";
  const mine = count(ownerV, a), theirs = count(ownerV, b);
  const themName = game.mode === "astralis" ? "astralis" : game.mode === "peer" ? (game.peerHandle ?? "them") : "cyan";
  const youName = solo ? "violet" : "you";
  const closedLast = lastV !== null && boxesOf(geo, lastV).some((x) => ownerV[x] !== 0);
  const name = levelName(levels, game.level);
  const nextOpen = !active && game.mode !== "peer" && unlocked !== null && unlocked > game.level && game.level < 5;

  return (
    <div>
      <header className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <Opponent game={game} />
        <span className="ml-auto font-mono text-[.5625rem] uppercase tracking-[.12em] text-ink-3">{name}</span>
      </header>

      {/* the score is the legend: which colour and shape is whose, in the same marks the board uses */}
      <ul aria-label={`score: ${youName} ${mine}, ${themName} ${theirs}`}
          className="mt-2.5 flex flex-wrap items-center gap-x-5 gap-y-1 font-mono text-[.75rem] text-ink-2">
        <li className="flex items-center gap-1.5">
          <Swatch tone={tone(a)} />{youName} <b className="font-medium text-ink">{mine}</b>
        </li>
        <li className="flex min-w-0 items-center gap-1.5">
          <Swatch tone={tone(b)} /><span className="truncate">{themName}</span> <b className="font-medium text-ink">{theirs}</b>
        </li>
      </ul>

      <p role="status" aria-live="polite"
         className="mt-3 min-h-[1.25rem] font-mono text-[.6875rem] text-ink-2">
        {active && turnLine()}
      </p>

      <div className="game-pane mt-2">
        <Board geo={geo} drawn={drawnV} owner={ownerV} drawnBy={v.drawnBy}
               lastEdge={lastV} run={game.mode === "astralis" ? run : []} tone={tone}
               interactive={active} live={canMove} onMove={(e) => void move(e)} />
      </div>

      {game.status !== "active" && (
        <section className="game-pane mt-5 animate-rise p-5 text-center">
          <p className="font-sans text-[1.0625rem] font-medium tracking-[-.01em] text-ink">{headline()}</p>
          <p className="mx-auto mt-1.5 max-w-measure text-[.8125rem] leading-relaxed text-ink-2">{subline()}</p>
          {nextOpen && (
            <p className="mt-2 font-mono text-[.6875rem] text-violet-2">{levelName(levels, game.level + 1)} is open</p>
          )}
          <div className="mt-4 flex flex-wrap justify-center gap-2">
            <Link href="/play" className="btn-quiet">Back to play</Link>
            {(game.mode !== "peer" || game.peerId) && (
              <button className={nextOpen ? "btn-quiet" : "btn-primary"} disabled={again}
                      onClick={() => void playAgain(game.level)}>
                {game.mode === "peer" ? `Invite ${game.peerHandle} again` : `${name} again`}
              </button>
            )}
            {nextOpen && (
              <button className="btn-primary" disabled={again} onClick={() => void playAgain(game.level + 1)}>
                Try {levelName(levels, game.level + 1)}
              </button>
            )}
          </div>
        </section>
      )}

      {notice && <p role="status" className="mt-4 text-center font-mono text-[.6875rem] text-ink-3">{notice}</p>}
      {error && <p role="alert" className="mt-4 text-center font-mono text-[.75rem] text-amber">{error}</p>}
    </div>
  );

  function turnLine(): string {
    if (hidden.length) return "astralis is joining stars…";
    if (solo) {
      const who = v.toMove === 1 ? "violet" : "cyan";
      return closedLast ? `a square closed: ${who} again` : `${who}'s move`;
    }
    if (game.myTurn) return closedLast && lastDrawer === game.me ? "a square closed: your move again" : "your move";
    return game.mode === "astralis" ? "astralis is joining stars…" : `${game.peerHandle ?? "their"}'s move`;
  }

  function headline(): string {
    if (game.status === "abandoned") return "this game went quiet";
    if (solo) return "every square is closed";
    if (game.result === "draw") return game.mode === "peer" ? "a shared sky: +2 points" : "a shared sky: +1 point";
    if (game.mode === "astralis") {
      return game.result === "won" ? "you closed more squares: +2 points" : "astralis closed more this time: +1 point";
    }
    return game.result === "won" ? "a win: +3 points" : "you finished: +1 point";
  }

  function subline(): string {
    if (game.status === "abandoned") return "A day passed without a move, so it was set down. Nothing is lost.";
    if (solo) return `Violet ${mine}, cyan ${theirs}. Both of them were you.`;
    if (game.result === "draw" && game.mode === "astralis") return `${mine} squares each. An even night, which is a nice way for one to end.`;
    if (game.mode === "astralis") return "Games with Astralis count towards your week, up to 10 practice points a day.";
    return `Thank you for playing with ${game.peerHandle}. Every finished game counts this week.`;
  }
}

function Opponent({ game }: { game: Game }) {
  if (game.mode === "astralis") {
    return (
      <span className="flex items-center gap-2">
        <AstralisMark className="!h-5 !w-5" />
        <span className="font-mono text-[.8125rem] text-ink">astralis</span>
      </span>
    );
  }
  if (game.mode === "solo") {
    return (
      <span className="flex items-center gap-2">
        <Butterfly pose="rest" className="h-4 w-4 text-violet-2/70" />
        <span className="font-mono text-[.8125rem] text-ink">solo</span>
        <span className="font-mono text-[.5625rem] text-ink-3">both colours, one screen</span>
      </span>
    );
  }
  return (
    <span className="flex min-w-0 items-center gap-2">
      <Butterfly pose="rest" className="h-4 w-4 shrink-0 text-violet-2/70" />
      <span className="truncate font-mono text-[.8125rem] text-ink">{game.peerHandle}</span>
    </span>
  );
}
