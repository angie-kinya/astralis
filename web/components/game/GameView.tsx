"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import Butterfly from "@/components/Butterflies";
import Board, { type Tone } from "@/components/game/Board";
import { EDGES, applyLocal, boxEdges, boxesOf, count } from "@/components/game/rules";
import { AstralisMark } from "@/components/Speaker";
import { IconBack } from "@/components/icons";
import {
  ApiError, type Game, type Player, createGame, getGame, playMove, sendInvite, userMessage,
} from "@/lib/api";
import { useEvent, useEventsConnected } from "@/lib/events";

const STEP = 350; // ms between astralis's lines when they are played back

const reducedMotion = () =>
  typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/**
 * one game of constellations, in any of its three modes.
 *
 *  - the server is the referee. a move is drawn at once (applyLocal) and
 *    posted with the version it was drawn against; a 409 means the board
 *    moved underneath us, so the tab refetches and quietly takes the server's
 *    word. nothing is ever offered while it is not this tab's turn.
 *  - game.update events are hints like every other event: they are taken only
 *    when newer than what is on screen, and a 60 s poll covers a dropped stream.
 *  - astralis answers inside the same request, so its lines arrive all at once
 *    in botEdges. they are hidden and revealed one by one, so its reply reads
 *    as moves rather than a jump (instantly under reduced motion).
 *  - the contract does not say who drew each edge, so drawers are remembered
 *    per tab from what this tab saw happen. edges from before it opened are
 *    drawn in neutral starlight rather than guessed
 */
export default function GameView({ gameId }: { gameId: string }) {
  const router = useRouter();
  const live = useEventsConnected();
  const [game, setGame] = useState<Game | null>(null);
  const [drawers, setDrawers] = useState<Player[]>(() => Array(EDGES).fill(0));
  const [hidden, setHidden] = useState<number[]>([]);
  const [missing, setMissing] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [again, setAgain] = useState(false);

  const gameRef = useRef<Game | null>(null);
  const drawersRef = useRef<Player[]>(drawers);
  const pendingRef = useRef(false);
  const timers = useRef<number[]>([]);
  // the last edge on screen before astralis's reply started, so the halo has somewhere to sit
  const beforeBot = useRef<number | null>(null);

  const clearTimers = () => { timers.current.forEach((t) => window.clearTimeout(t)); timers.current = []; };
  useEffect(() => clearTimers, []);

  const adopt = useCallback((next: Game, force = false) => {
    const prev = gameRef.current;
    if (!force && prev && next.version <= prev.version) return;

    let d: Player[];
    if (next.drawnBy && next.drawnBy.length === EDGES) {
      d = next.drawnBy.slice();
    } else {
      d = drawersRef.current.slice();
      const bot = new Set(next.botEdges);
      for (let e = 0; e < EDGES; e++) {
        if (!next.drawn[e]) { d[e] = 0; continue; }
        if (bot.has(e)) d[e] = (3 - next.me) as Player;
        // new since the last state this tab saw: whoever was to move then drew it. a run
        // of several only happens when one side keeps closing squares, so it holds for all
        else if (prev && !prev.drawn[e] && !d[e]) d[e] = prev.toMove;
      }
    }
    drawersRef.current = d;
    setDrawers(d);

    clearTimers();
    const queue = reducedMotion() ? [] : next.botEdges.filter((e) => next.drawn[e]);
    beforeBot.current = prev?.lastEdge ?? null;
    setHidden(queue);
    queue.forEach((_, i) => {
      timers.current.push(window.setTimeout(() => setHidden((h) => h.slice(1)), STEP * (i + 1)));
    });

    gameRef.current = next;
    setGame(next);
  }, []);

  const load = useCallback(async () => {
    try {
      const { game } = await getGame(gameId);
      adopt(game, true);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) setMissing(true);
      else setError(userMessage(e));
    }
  }, [gameId, adopt]);

  useEffect(() => { void load(); }, [load]);

  useEvent<{ game: Game }>("game.update", ({ game }) => {
    if (game?.id === gameId) adopt(game);
  });

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

  const canMove = !!game && active && game.myTurn && !pending && hidden.length === 0;

  async function move(e: number) {
    const g = gameRef.current;
    if (!g || g.status !== "active" || !g.myTurn || g.drawn[e] || pendingRef.current || hidden.length) return;
    pendingRef.current = true; setPending(true); setError(null); setNotice(null);

    const d = drawersRef.current.slice();
    d[e] = g.mode === "solo" ? g.toMove : g.me;
    drawersRef.current = d; setDrawers(d);
    const optimistic = applyLocal(g, e);
    gameRef.current = optimistic; setGame(optimistic);

    try {
      const { game: next } = await playMove(g.id, e, g.version);
      adopt(next);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setNotice("the board moved on before that landed. here it is as it stands.");
      } else {
        setError(userMessage(err));
      }
      await load();
    } finally {
      pendingRef.current = false; setPending(false);
    }
  }

  async function playAgain() {
    if (!game) return;
    setAgain(true); setError(null); setNotice(null);
    try {
      if (game.mode === "peer" && game.peerId) {
        await sendInvite(game.peerId);
        setNotice(`invited. it waits in your thread with ${game.peerHandle}.`);
      } else if (game.mode !== "peer") {
        const { game: g } = await createGame(game.mode);
        router.push(`/circle/play/${g.id}`);
        return;
      }
    } catch (e) {
      if (e instanceof ApiError && e.code === "invite_pending") {
        setNotice(`there is already an invite waiting with ${game.peerHandle}.`);
      } else setError(userMessage(e));
    }
    setAgain(false);
  }

  if (missing) {
    return (
      <Frame>
        <p className="font-mono text-[.75rem] leading-relaxed text-ink-3">
          this game is not here. it may belong to someone else, or the link is wrong.
        </p>
      </Frame>
    );
  }
  if (!game) {
    return (
      <Frame>
        {error
          ? <p role="alert" className="font-mono text-[.75rem] text-amber">{error}</p>
          : <p className="font-mono text-[.75rem] text-ink-3">gathering the stars…</p>}
      </Frame>
    );
  }

  // what is on screen: the game minus any of astralis's lines still waiting to be revealed
  const hide = new Set(hidden);
  const drawnV = game.drawn.map((d, e) => d && !hide.has(e));
  const ownerV = game.owner.map((o, b) => (boxEdges(b).every((e) => drawnV[e]) ? o : 0)) as Player[];
  const shown = game.botEdges.length - hidden.length;
  const lastV = hidden.length === 0 ? game.lastEdge
    : shown > 0 ? game.botEdges[shown - 1]! : beforeBot.current;
  const lastDrawer: Player = lastV === null ? 0 : (drawers[lastV] || derive(game, lastV));

  const other = (3 - game.me) as Player;
  const tone = (p: Player): Tone =>
    game.mode === "solo" ? (p === 1 ? "you" : "them")
      : p === game.me ? "you" : game.mode === "astralis" ? "bot" : "them";
  const mine = count(ownerV, game.mode === "solo" ? 1 : game.me);
  const theirs = count(ownerV, game.mode === "solo" ? 2 : other);
  const themName = game.mode === "astralis" ? "astralis" : "them";

  const closedLast = lastV !== null && boxesOf(lastV).some((b) => ownerV[b] !== 0);

  return (
    <Frame>
      <header className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <Opponent game={game} />
        <p className="ml-auto flex items-center gap-2 font-mono text-[.75rem] text-ink-2"
           aria-label={game.mode === "solo"
             ? `violet ${mine}, cyan ${theirs}`
             : `you ${mine}, ${themName} ${theirs}`}>
          <span className="flex items-center gap-1.5">
            <span className="game-swatch gp-you" aria-hidden />{game.mode === "solo" ? "violet" : "you"}{" "}
            <b className="font-medium text-ink">{mine}</b>
          </span>
          <span className="text-ink-3" aria-hidden>·</span>
          <span className="flex items-center gap-1.5">
            <span className={`game-swatch gp-${tone(game.mode === "solo" ? 2 : other)}`} aria-hidden />
            {game.mode === "solo" ? "cyan" : themName} <b className="font-medium text-ink">{theirs}</b>
          </span>
        </p>
      </header>

      <p role="status" aria-live="polite"
         className="mt-3 min-h-[1.25rem] font-mono text-[.6875rem] text-ink-3">
        {active && turnLine()}
      </p>

      <div className="mt-4">
        <Board drawn={drawnV} owner={ownerV} drawers={drawers}
               lastEdge={lastV} lastDrawer={lastDrawer} tone={tone}
               interactive={active} live={canMove} onMove={(e) => void move(e)} />
      </div>

      {game.status !== "active" && (
        <section className="card mt-6 animate-rise p-5 text-center">
          <p className="font-sans text-[1.0625rem] font-medium tracking-[-.01em] text-ink">{headline()}</p>
          <p className="mx-auto mt-1.5 max-w-measure text-[.8125rem] leading-relaxed text-ink-2">{subline()}</p>
          <div className="mt-4 flex flex-wrap justify-center gap-2">
            <Link href="/circle" className="btn-quiet">Back to the circle</Link>
            {(game.mode !== "peer" || game.peerId) && (
              <button className="btn-primary" disabled={again} onClick={() => void playAgain()}>
                {game.mode === "peer" ? `Invite ${game.peerHandle} again` : "Play again"}
              </button>
            )}
          </div>
        </section>
      )}

      {notice && <p role="status" className="mt-4 text-center font-mono text-[.6875rem] text-ink-3">{notice}</p>}
      {error && <p role="alert" className="mt-4 text-center font-mono text-[.75rem] text-amber">{error}</p>}
    </Frame>
  );

  function turnLine(): string {
    if (!game) return "";
    if (hidden.length) return "astralis is joining stars…";
    if (game.mode === "solo") {
      const who = game.toMove === 1 ? "violet" : "cyan";
      return closedLast ? `a square closed: ${who} again` : `${who}'s move`;
    }
    if (game.myTurn) return closedLast && lastDrawer === game.me ? "a square closed: your move again" : "your move";
    return game.mode === "astralis" ? "astralis is joining stars…" : "their move";
  }

  function headline(): string {
    if (!game) return "";
    if (game.status === "abandoned") return "this game went quiet";
    if (game.mode === "solo") return "all nine squares are closed";
    if (game.mode === "astralis") {
      return game.result === "won" ? "you closed more squares this time" : "astralis closed more squares this time";
    }
    return game.result === "won" ? "a win: +3 points" : "you finished: +1 point";
  }

  function subline(): string {
    if (!game) return "";
    if (game.status === "abandoned") return "A day passed without a move, so it was set down. Nothing is lost.";
    if (game.mode === "solo") return `Violet ${mine}, cyan ${theirs}. Both of them were you.`;
    if (game.mode === "astralis") return "Games with Astralis are just for you. They never touch the weekly board.";
    return `Thank you for playing with ${game.peerHandle}. Every finished game counts this week.`;
  }
}

/**
 * who drew the last edge when this tab did not see it happen. any square next
 * to it that is owned was completed by it (it was the missing side), so the
 * owner drew it; otherwise the turn passed, so it was the player not to move
 */
function derive(g: Game, e: number): Player {
  if (g.lastEdge !== e) return 0;
  const closed = boxesOf(e).find((b) => g.owner[b]);
  if (closed !== undefined) return g.owner[closed]!;
  return g.status === "active" ? ((3 - g.toMove) as Player) : 0;
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

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-xl px-4 py-5 sm:py-7">
        <div className="mb-5 flex items-center justify-between">
          <Link href="/circle"
                className="-ml-1 flex items-center gap-1 rounded-lg px-1 py-0.5 font-mono text-[.6875rem]
                           text-ink-3 transition-colors hover:text-ink">
            <IconBack className="h-3.5 w-3.5" />
            circle
          </Link>
          <span className="label">constellations</span>
        </div>
        {children}
      </div>
    </div>
  );
}
