"use client";

import { useId } from "react";

// what the next hint will be, in the order the server gives them
const NEXT = ["a clue", "one of the letters", "the first and last letters"];

/**
 * hints are free and never counted against anyone. the drawer opens in
 * place under the board (no overlay to dismiss on a phone), shows what has
 * been revealed so far, and offers the next one plainly
 */
export default function HintDrawer({ open, onToggle, hints, hintsLeft, canAsk, asking, onAsk }: {
  open: boolean;
  onToggle: () => void;
  hints: string[];
  hintsLeft: number;
  canAsk: boolean;
  asking: boolean;
  onAsk: () => void;
}) {
  const id = useId();
  const next = NEXT[hints.length] ?? "another nudge";

  return (
    <div className="rounded-xl border border-line bg-surface/60">
      <button type="button" aria-expanded={open} aria-controls={id} onClick={onToggle}
              className="flex w-full items-center justify-between gap-3 rounded-xl px-3.5 py-2.5 text-left">
        <span className="flex items-center gap-2 font-mono text-[.75rem] text-ink-2">
          <IconLantern className="h-4 w-4 text-amber" />
          {hints.length ? `hints (${hints.length})` : "a hint, if you like"}
        </span>
        <span className="font-mono text-[.625rem] text-ink-3" aria-hidden>{open ? "close" : "open"}</span>
      </button>

      {open && (
        <div id={id} className="animate-fade border-t border-line px-3.5 pb-3.5 pt-3">
          {hints.length > 0 && (
            <ol className="space-y-2" aria-live="polite">
              {hints.map((h, i) => (
                <li key={i} className="flex gap-2.5 text-[.8125rem] leading-relaxed text-ink">
                  <span className="mt-[.2rem] font-mono text-[.625rem] text-ink-3">{i + 1}</span>
                  <span>{h}</span>
                </li>
              ))}
            </ol>
          )}
          {canAsk && hintsLeft > 0 ? (
            <div className={`flex flex-wrap items-center gap-x-3 gap-y-2 ${hints.length ? "mt-3" : ""}`}>
              <button type="button" className="btn-quiet !py-2" disabled={asking} onClick={onAsk}>
                Show {next}
              </button>
              <span className="font-mono text-[.625rem] text-ink-3">free, as many as there are</span>
            </div>
          ) : (
            <p className={`font-mono text-[.6875rem] text-ink-3 ${hints.length ? "mt-3" : ""}`}>
              {hintsLeft > 0 ? "hints rest once the puzzle is done." : "that is every hint for this one."}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function IconLantern({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5}
         strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden>
      <path d="M9.5 18h5M10 20.5h4" />
      <path d="M12 3.5a5.5 5.5 0 0 0-3.2 10c.6.5 1 1.2 1 2V16h4.4v-.5c0-.8.4-1.5 1-2A5.5 5.5 0 0 0 12 3.5Z" />
    </svg>
  );
}
