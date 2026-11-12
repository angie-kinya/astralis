"use client";

import { useState } from "react";

import type { Source } from "@/lib/api";

/**
 * A real citation apparatus, not a row of chips.
 *
 * Collapsed by default — the prose should be read first. Expanded it reads as
 * a reference list, with the retrieval score exposed. Showing the score is
 * deliberate: this is a research tool, and someone who can see a passage
 * scored 0.41 can tell you the grounding was thin. Hiding it would be
 * flattering the system.
 *
 * the server only sends sources a reply actually cited, so a vent with no
 * citations shows nothing. collapsed it is one small chip, "2 studies", so
 * a conversation never reads like a literature review.
 */
export default function SourceList({ sources }: { sources: Source[] }) {
  const [open, setOpen] = useState(false);
  if (!sources.length) return null;
  // two passages from one paper are one study to the reader
  const studies = new Set(sources.map((s) => s.url || s.title)).size;

  return (
    <div className="mt-1">
      <button onClick={() => setOpen((v) => !v)} aria-expanded={open}
              className="tag inline-flex items-center gap-1.5 text-ink-3 transition-colors
                         hover:border-violet/50 hover:text-ink-2">
        <svg viewBox="0 0 12 12" aria-hidden
             className={`h-2.5 w-2.5 transition-transform duration-200 ease-swift ${open ? "rotate-90" : ""}`}>
          <path d="M4 2.5 8 6l-4 3.5" fill="none" stroke="currentColor" strokeWidth="1.4"
                strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {studies} {studies === 1 ? "study" : "studies"}
      </button>

      {open && (
        <ol className="mt-3 space-y-2.5 border-l border-line pl-4 animate-fade">
          {sources.map((s, i) => (
            <li key={s.id} className="flex gap-3 text-[.8125rem] leading-snug">
              <span className="mt-px font-mono text-[.625rem] tabular-nums text-violet-2">
                {String(i + 1).padStart(2, "0")}
              </span>
              <span className="min-w-0">
                <a href={s.url} target="_blank" rel="noreferrer noopener"
                   className="font-sans text-ink-2 underline decoration-line underline-offset-2
                              transition-colors hover:text-ink hover:decoration-violet">
                  {s.title}
                </a>
                <span className="ml-2 font-mono text-[.625rem] tabular-nums text-ink-3">
                  {s.year ?? "n.d."} · {s.score.toFixed(2)}
                </span>
              </span>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
