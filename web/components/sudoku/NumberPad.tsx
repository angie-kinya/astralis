"use client";

import { IconErase } from "@/components/words/WordsKeyboard";

/**
 * digits plus three tools: notes, erase, undo. a digit that is already
 * placed everywhere it can be fades a little, as a quiet sign it is done
 */
export default function NumberPad({ size, placed, notesMode, canEdit, canUndo, onDigit, onErase, onNotes, onUndo }: {
  size: number;
  placed: number[];
  notesMode: boolean;
  canEdit: boolean;
  canUndo: boolean;
  onDigit: (d: number) => void;
  onErase: () => void;
  onNotes: () => void;
  onUndo: () => void;
}) {
  const key = "flex h-12 min-w-0 flex-1 items-center justify-center rounded-lg border font-mono transition-colors " +
              "duration-150 [touch-action:manipulation] active:translate-y-px disabled:opacity-40";
  return (
    <div className="mx-auto w-full max-w-[28rem] select-none space-y-2">
      <div className="flex gap-1.5" role="group" aria-label={notesMode ? "digits, notes on" : "digits"}>
        {Array.from({ length: size }, (_, k) => {
          const d = k + 1;
          const done = (placed[d] ?? 0) >= size;
          return (
            <button key={d} type="button" disabled={!canEdit} onClick={() => onDigit(d)}
                    aria-label={notesMode ? `note ${d}` : `${d}${done ? ", all placed" : ""}`}
                    className={`${key} text-[1.0625rem] ${notesMode ? "border-violet/45 bg-violet/[.08] text-violet-2" : "border-line bg-surface/70 text-ink hover:border-line-2 hover:bg-raised"}
                                ${done && !notesMode ? "opacity-45" : ""}`}>
              {d}
            </button>
          );
        })}
      </div>
      <div className="flex gap-1.5">
        <button type="button" aria-pressed={notesMode} onClick={onNotes}
                className={`${key} gap-1.5 text-[.6875rem] ${notesMode ? "border-violet/60 bg-violet/15 text-ink" : "border-line bg-surface/70 text-ink-2 hover:bg-raised"}`}>
          <IconPencil className="h-4 w-4" /> notes {notesMode ? "on" : "off"}
        </button>
        <button type="button" disabled={!canEdit} onClick={onErase}
                className={`${key} gap-1.5 border-line bg-surface/70 text-[.6875rem] text-ink-2 hover:bg-raised`}>
          <IconErase className="h-4 w-4" /> erase
        </button>
        <button type="button" disabled={!canUndo} onClick={onUndo}
                className={`${key} gap-1.5 border-line bg-surface/70 text-[.6875rem] text-ink-2 hover:bg-raised`}>
          <IconUndo className="h-4 w-4" /> undo
        </button>
      </div>
    </div>
  );
}

const stroke = {
  viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: 1.5,
  strokeLinecap: "round" as const, strokeLinejoin: "round" as const,
};

function IconPencil({ className }: { className?: string }) {
  return (
    <svg {...stroke} className={className} aria-hidden>
      <path d="M15.5 5.5 18.5 8.5 9 18l-4 1 1-4Z" /><path d="m13.5 7.5 3 3" opacity=".6" />
    </svg>
  );
}

function IconUndo({ className }: { className?: string }) {
  return (
    <svg {...stroke} className={className} aria-hidden>
      <path d="M9 7.5 5 11.5l4 4" /><path d="M5.5 11.5H14a4.5 4.5 0 0 1 0 9h-2" />
    </svg>
  );
}
