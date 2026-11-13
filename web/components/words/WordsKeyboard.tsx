"use client";

import { type Mark, markStyle, markText, markWord } from "@/components/words/marks";

const ROWS = [
  ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"],
  ["A", "S", "D", "F", "G", "H", "J", "K", "L"],
  ["ENTER", "Z", "X", "C", "V", "B", "N", "M", "BACK"],
];

/**
 * the on-screen keys. every key is a real button; data-wkey lets the page's
 * physical keyboard listener tell a focused key apart from other buttons, so
 * enter after a tap submits the guess instead of pressing that letter again
 */
export default function WordsKeyboard({ letters, disabled, onKey }: {
  letters: Record<string, Mark>;
  disabled: boolean;
  onKey: (key: string) => void;
}) {
  return (
    <div className="mx-auto flex w-full max-w-md select-none flex-col gap-1.5" aria-label="keyboard" role="group">
      {ROWS.map((row, i) => (
        <div key={i} className="flex gap-1 sm:gap-1.5">
          {i === 1 && <span className="flex-[.5]" aria-hidden />}
          {row.map((k) => {
            const wide = k === "ENTER" || k === "BACK";
            const mark = letters[k];
            const label = k === "ENTER" ? "submit guess" : k === "BACK" ? "delete letter"
              : mark ? `${k}, ${markWord[mark]}` : k;
            return (
              <button key={k} type="button" data-wkey disabled={disabled}
                      aria-label={label}
                      onClick={() => onKey(k)}
                      style={mark ? markStyle[mark] : undefined}
                      className={`flex h-12 min-w-0 items-center justify-center rounded-lg border font-mono
                                  transition-[transform,filter,background-color] duration-150 ease-swift
                                  [touch-action:manipulation] active:translate-y-px disabled:opacity-50
                                  ${wide ? "flex-[1.6] text-[.625rem] uppercase tracking-[.12em]" : "flex-1 text-[.8125rem]"}
                                  ${mark ? `${markText[mark]} hover:brightness-110`
                                         : "border-line bg-surface/70 text-ink hover:border-line-2 hover:bg-raised"}`}>
                {k === "BACK" ? <IconErase className="h-4 w-4" /> : k === "ENTER" ? "enter" : k}
              </button>
            );
          })}
          {i === 1 && <span className="flex-[.5]" aria-hidden />}
        </div>
      ))}
    </div>
  );
}

export function IconErase({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5}
         strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden>
      <path d="M9 5.5h10a1.5 1.5 0 0 1 1.5 1.5v10a1.5 1.5 0 0 1-1.5 1.5H9L3.5 12Z" />
      <path d="m11.5 9.5 5 5M16.5 9.5l-5 5" />
    </svg>
  );
}
