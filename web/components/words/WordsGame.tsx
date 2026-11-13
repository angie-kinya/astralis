"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import HintDrawer from "@/components/words/HintDrawer";
import WordsBoard from "@/components/words/WordsBoard";
import WordsKeyboard from "@/components/words/WordsKeyboard";
import WordsResult from "@/components/words/WordsResult";
import { shake } from "@/components/words/marks";
import { ApiError, type CreateOptions, type Game, type WordsView, createGame, getGame, playMove, userMessage } from "@/lib/api";
import { useReducedMotion } from "@/lib/motion";

const FLIP = 220; // ms between tiles turning over

const MODE_NAME: Record<WordsView["mode"], string> = { classic: "classic", anagram: "anagram", mystery: "mystery" };

/**
 * one words puzzle, solo or a duel.
 *
 *  - the server holds the word. a guess is checked for length here (a gentle
 *    shake, no request) and otherwise posted with the version it was typed
 *    against; a 409 means another tab moved first, so the game is refetched
 *    and handed up, and the letters typed stay put to try again.
 *  - the newest row turns over one tile at a time, instantly under reduced
 *    motion. the result card waits for the last tile.
 *  - physical typing works anywhere on the page except inside text fields.
 *  - a duel shows the other person's guess count and whether they solved it,
 *    never their letters
 */
export default function WordsGame({ game, onGame }: { game: Game; onGame: (g: Game) => void }) {
  const router = useRouter();
  const reduced = useReducedMotion();
  const view = game.view as WordsView;

  const [typed, setTyped] = useState("");
  const [pending, setPending] = useState(false);
  const [asking, setAsking] = useState(false);
  const [hintsOpen, setHintsOpen] = useState(false);
  const [status, setStatus] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [again, setAgain] = useState(false);
  const [reveal, setReveal] = useState<{ row: number; shown: number } | null>(null);

  const rowRef = useRef<HTMLDivElement>(null);
  const gameRef = useRef(game);
  gameRef.current = game;
  const typedRef = useRef(typed);
  typedRef.current = typed;
  const busy = useRef(false);

  const letters = useMemo(() => {
    const out: WordsView["letters"] = {};
    for (const [k, v] of Object.entries(view.letters ?? {})) out[k.toUpperCase()] = v;
    return out;
  }, [view.letters]);

  const active = game.status === "active" && !view.over;
  const canType = active && !pending;

  useEffect(() => { setTyped(""); setHintsOpen(false); setStatus(""); setError(null); }, [game.id]);

  // reveal the newest row tile by tile when it arrives while this board is open
  const seen = useRef({ id: game.id, n: view.guesses.length });
  const count = view.guesses.length;
  useEffect(() => {
    if (seen.current.id !== game.id) { seen.current = { id: game.id, n: count }; return; }
    const before = seen.current.n;
    seen.current.n = count;
    if (count <= before || reduced) return;
    const row = count - 1;
    let shown = 1;
    setReveal({ row, shown });
    const t = window.setInterval(() => {
      shown += 1;
      if (shown > view.length) { window.clearInterval(t); setReveal(null); }
      else setReveal({ row, shown });
    }, FLIP);
    return () => { window.clearInterval(t); setReveal(null); };
  }, [game.id, count, view.length, reduced]);

  const send = useCallback(async (move: { guess: string } | { hint: true }) => {
    const g = gameRef.current;
    if (busy.current) return false;
    busy.current = true; setError(null);
    try {
      const { game: next } = await playMove(g.id, move, g.version);
      // cleared in the same render as the new row lands, so the letters never flash on the next one
      if ("guess" in move) setTyped("");
      onGame(next);
      return true;
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setStatus("this puzzle moved on in another tab. here it is as it stands.");
        try { onGame((await getGame(g.id)).game); } catch (err) { setError(userMessage(err)); }
      } else {
        setError(userMessage(e));
      }
      return false;
    } finally {
      busy.current = false;
    }
  }, [onGame]);

  const submit = useCallback(async () => {
    const word = typedRef.current;
    const len = (gameRef.current.view as WordsView).length;
    if (word.length !== len) {
      shake(rowRef.current, reduced);
      setStatus(`this one needs ${len} letters${word.length ? `, you have ${word.length}` : ""}.`);
      return;
    }
    setPending(true); setStatus("");
    await send({ guess: word });
    setPending(false);
  }, [reduced, send]);

  const press = useCallback((k: string) => {
    if (!canType) return;
    const len = view.length;
    if (k === "ENTER") { void submit(); return; }
    if (k === "BACK") { setTyped((t) => t.slice(0, -1)); return; }
    if (/^[A-Z]$/.test(k)) {
      setTyped((t) => (t.length < len ? t + k : t));
      setStatus("");
    }
  }, [canType, view.length, submit]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey || e.isComposing) return;
      const t = e.target instanceof HTMLElement ? e.target : null;
      if (t?.closest("input, textarea, select, [contenteditable='true']")) return;
      const onKeyButton = !!t?.closest("[data-wkey]");
      const onOtherControl = !onKeyButton && !!t?.closest("button, a, summary");
      let k: string | null = null;
      if (e.key === "Enter") k = onOtherControl ? null : "ENTER";
      else if (e.key === "Backspace") k = "BACK";
      else if (/^[a-z]$/i.test(e.key)) k = e.key.toUpperCase();
      if (!k) return;
      e.preventDefault();
      press(k);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [press]);

  async function hint() {
    setAsking(true);
    await send({ hint: true });
    setAsking(false);
  }

  async function another() {
    setAgain(true); setError(null);
    try {
      const options: CreateOptions = { mode: view.mode };
      if (game.options.topic) options.topic = game.options.topic;
      const { game: g } = await createGame("words", "solo", options);
      router.push(`/play/${g.id}`);
    } catch (e) {
      setError(userMessage(e));
      setAgain(false);
    }
  }

  const done = !active && !reveal;
  const daily = !!game.options.day;

  return (
    <div>
      <header className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
        <span className="tag">{MODE_NAME[view.mode]}</span>
        <span className="font-mono text-[.8125rem] text-ink">{view.topic ?? (view.over ? "" : "topic hidden")}</span>
        <span className="ml-auto font-mono text-[.625rem] text-ink-3">
          {game.mode === "peer" ? `duel with ${game.peerHandle ?? "a friend"}` : daily ? "today's puzzle" : "practice"}
        </span>
      </header>

      {game.mode === "peer" && view.opponent && <Opponent handle={game.peerHandle} o={view.opponent} />}

      <section className="bubble mt-3 border-line bg-surface/80 !px-3 !py-4 sm:!px-5 sm:!py-5">
        {view.mode === "anagram" && view.scrambled && (
          <div className="mb-4 text-center">
            <p className="label">the letters</p>
            <p className="mt-2 flex flex-wrap justify-center gap-1" aria-label={`scrambled: ${view.scrambled.split("").join(" ")}`}>
              {view.scrambled.toUpperCase().split("").map((l, i) => (
                <span key={i} aria-hidden
                      className="flex h-7 w-7 items-center justify-center rounded-md border border-line-2 bg-raised/60
                                 font-mono text-[.8125rem] text-ink-2">{l}</span>
              ))}
            </p>
          </div>
        )}

        <WordsBoard ref={rowRef} length={view.length} maxGuesses={view.maxGuesses} guesses={view.guesses}
                    typed={typed} live={active} pending={pending} reveal={reveal} reduced={reduced} />

        <p role="status" aria-live="polite" className="mt-3 min-h-[1.25rem] text-center font-mono text-[.6875rem] text-ink-3">
          {status || (pending ? "checking…" : "")}
        </p>

        {active && (
          <div className="mt-1">
            <WordsKeyboard letters={letters} disabled={!canType} onKey={press} />
          </div>
        )}
      </section>

      {(active || view.hints.length > 0) && (
        <div className="mt-3">
          <HintDrawer open={hintsOpen} onToggle={() => setHintsOpen((o) => !o)} hints={view.hints}
                      hintsLeft={view.hintsLeft} canAsk={active} asking={asking} onAsk={() => void hint()} />
        </div>
      )}

      {done && <WordsResult game={game} view={view} busy={again} onAnother={() => void another()} />}

      {error && <p role="alert" className="mt-4 text-center font-mono text-[.75rem] text-amber">{error}</p>}
    </div>
  );
}

function Opponent({ handle, o }: { handle: string | null; o: NonNullable<WordsView["opponent"]> }) {
  const line = o.over ? (o.solved ? `solved in ${o.guesses}` : "finished") : o.guesses ? `${o.guesses} ${o.guesses === 1 ? "guess" : "guesses"} so far` : "not started yet";
  return (
    <p className="mt-2 flex items-center gap-2 font-mono text-[.6875rem] text-ink-3" aria-live="polite">
      <span className="game-swatch shrink-0" style={{ background: "rgb(var(--p-them, var(--amber)))" }} aria-hidden />
      <span className="truncate text-ink-2">{handle ?? "your friend"}</span>
      <span>{line}</span>
    </p>
  );
}
