"use client";

import Link from "next/link";

import type { Game, WordsView } from "@/lib/api";

const YOU = "rgb(var(--p-you, var(--violet)))";

/**
 * the end of a words puzzle: the word, its clue and its fact, and how many
 * guesses it took. no streaks, no scores to beat. in a duel the comparison
 * waits until both people are done, and before that only says they are still
 * working on it
 */
export default function WordsResult({ game, view, busy, onAnother }: {
  game: Game; view: WordsView; busy: boolean; onAnother: () => void;
}) {
  const used = view.guesses.length;
  const duel = game.mode === "peer" && view.opponent;
  const them = game.peerHandle ?? "your friend";

  return (
    <section className="card mt-5 animate-rise p-5 text-center" aria-live="polite">
      <p className="label">{view.solved ? "found" : "the word was"}</p>
      {view.answer && (
        <p className="mt-3 flex flex-wrap justify-center gap-1" aria-label={view.answer}>
          {view.answer.toUpperCase().split("").map((l, i) => (
            <span key={i} aria-hidden
                  className="flex h-8 w-8 items-center justify-center rounded-md border font-mono text-[.9375rem]
                             font-medium text-white"
                  style={{ background: YOU, borderColor: YOU }}>
              {l}
            </span>
          ))}
        </p>
      )}

      <p className="mt-4 font-sans text-[1.0625rem] font-medium tracking-[-.01em] text-ink">{headline()}</p>
      {view.clue && <p className="mx-auto mt-2 max-w-measure text-[.8125rem] leading-relaxed text-ink-2">{view.clue}</p>}
      {view.fact && (
        <p className="mx-auto mt-3 max-w-measure rounded-lg border border-line bg-raised/50 px-3.5 py-2.5 text-left
                      text-[.8125rem] leading-relaxed text-ink-2">
          <span className="label mr-2">did you know</span>{view.fact}
        </p>
      )}

      {duel && view.opponent && (
        <div className="mx-auto mt-4 max-w-measure rounded-lg border border-line px-3.5 py-3 text-left">
          {view.opponent.over ? (
            <>
              <dl className="grid grid-cols-[1fr_auto] gap-x-4 gap-y-1 font-mono text-[.75rem]">
                <dt className="text-ink-2">you</dt>
                <dd className="text-ink">{view.solved ? guesses(used) : "not this time"}</dd>
                <dt className="truncate text-ink-2">{them}</dt>
                <dd className="text-ink">{view.opponent.solved ? guesses(view.opponent.guesses) : "not this time"}</dd>
              </dl>
              <p className="mt-2.5 text-[.8125rem] leading-relaxed text-ink-2">{duelLine()}</p>
            </>
          ) : (
            <p className="font-mono text-[.6875rem] leading-relaxed text-ink-3">
              {them} is still working on it. the comparison appears here once they are done.
            </p>
          )}
        </div>
      )}

      <div className="mt-5 flex flex-wrap justify-center gap-2">
        <Link href="/play" className="btn-quiet">Back to play</Link>
        <button type="button" className="btn-primary" disabled={busy} onClick={onAnother}>Another one</button>
      </div>
    </section>
  );

  function headline(): string {
    if (game.status === "abandoned") return "this one was set down. nothing is lost";
    if (view.solved) return used === 1 ? "first guess. lovely" : `found in ${used} guesses`;
    return "it stayed hidden this time";
  }

  function duelLine(): string {
    if (game.result === "won") return "you found it in fewer guesses. thank you both for playing.";
    if (game.result === "lost") return `${them} found it in fewer guesses. thank you both for playing.`;
    return "level pegging. thank you both for playing.";
  }
}

const guesses = (n: number) => `${n} ${n === 1 ? "guess" : "guesses"}`;
