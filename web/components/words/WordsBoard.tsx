"use client";

import { forwardRef, useEffect, useRef } from "react";

import { type Mark, markStyle, markText, markWord } from "@/components/words/marks";

type Row = { word: string; marks: Mark[] };

/**
 * the tile grid. revealed rows come from the server; the row being typed is
 * local. `reveal` holds back the newest row's marks so they turn over one tile
 * at a time (the parent drives the count, and skips it under reduced motion)
 */
const WordsBoard = forwardRef<HTMLDivElement, {
  length: number;
  maxGuesses: number;
  guesses: Row[];
  typed: string;
  /** typing is possible: the current row is drawn as the live one */
  live: boolean;
  /** the guess is on its way */
  pending: boolean;
  reveal: { row: number; shown: number } | null;
  reduced: boolean;
}>(function WordsBoard({ length, maxGuesses, guesses, typed, live, pending, reveal, reduced }, currentRef) {
  const rows = Math.max(maxGuesses, guesses.length);
  const size = length <= 5 ? "text-xl sm:text-2xl" : length <= 6 ? "text-lg sm:text-xl" : "text-base sm:text-lg";

  return (
    <div role="group" aria-label={`${guesses.length} of ${maxGuesses} guesses used`}
         className="mx-auto flex w-full flex-col gap-1.5 sm:gap-2"
         style={{ maxWidth: `${length * 3.6}rem` }}>
      {Array.from({ length: rows }, (_, r) => {
        const entry = guesses[r];
        const current = !entry && r === guesses.length && live;
        const letters = entry ? entry.word.toUpperCase() : current ? typed : "";
        return (
          <div key={r} ref={current ? currentRef : undefined}
               className="relative grid gap-1.5 sm:gap-2"
               style={{ gridTemplateColumns: `repeat(${length}, minmax(0, 1fr))` }}
               aria-hidden={!entry && !current ? true : undefined}>
            {(entry || current) && (
              <span className="sr-only">
                {entry
                  ? `guess ${r + 1}: ${rowLabel(entry)}`
                  : `typing: ${typed.split("").join(" ") || "nothing yet"}, ${length} letters`}
              </span>
            )}
            {Array.from({ length }, (_, c) => {
              const held = reveal && reveal.row === r && c >= reveal.shown;
              return (
                <Tile key={c} letter={letters[c] ?? ""} size={size} reduced={reduced}
                      mark={entry && !held ? entry.marks[c] : undefined}
                      typing={current} pending={current && pending} />
              );
            })}
          </div>
        );
      })}
    </div>
  );
});

export default WordsBoard;

function rowLabel(entry: Row): string {
  const w = entry.word.toUpperCase();
  return w.split("").map((l, i) => `${l} ${markWord[entry.marks[i] ?? "absent"]}`).join(", ");
}

function Tile({ letter, mark, typing, pending, size, reduced }: {
  letter: string; mark?: Mark; typing: boolean; pending: boolean; size: string; reduced: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const was = useRef<Mark | undefined>(mark);

  // turn over only when a mark arrives on a tile already on screen; a board
  // loaded fresh is simply there
  useEffect(() => {
    const el = ref.current;
    if (mark && !was.current && !reduced && el && typeof el.animate === "function") {
      el.animate(
        [{ transform: "rotateX(90deg)" }, { transform: "rotateX(0)" }],
        { duration: 300, easing: "cubic-bezier(.2,.6,.2,1)" },
      );
    }
    was.current = mark;
  }, [mark, reduced]);

  const base = "flex aspect-square items-center justify-center rounded-lg border font-mono font-medium uppercase " +
               "transition-colors duration-150 " + size;
  if (mark) {
    return <div ref={ref} aria-hidden className={`${base} ${markText[mark]}`} style={markStyle[mark]}>{letter}</div>;
  }
  const look = letter
    ? `border-violet/55 bg-raised/70 text-ink ${pending ? "opacity-70" : ""}`
    : typing ? "border-line-2 bg-surface/60" : "border-line/70 bg-surface/30";
  return <div ref={ref} aria-hidden className={`${base} ${look}`}>{letter}</div>;
}
