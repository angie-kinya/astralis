"use client";

import { useEffect, useState } from "react";

import { lastTime, type Fact } from "@/lib/api";
import { dayLabel } from "@/lib/format";

/**
 * a quiet line at the head of a fresh conversation: "last time · yesterday:
 * the audit moved to friday". drawn from person_facts, so it is something
 * they actually said, not a generic note. it feels attended to rather than
 * generated, and it is dismissible because being remembered should never
 * feel like being watched.
 *
 * only ever rendered on a fresh chat, never inside an existing one.
 */
export default function LastTime() {
  const [fact, setFact] = useState<Fact | null>(null);
  const [gone, setGone] = useState(false);

  useEffect(() => {
    let live = true;
    // best effort: a missing line is not worth an error on screen
    lastTime().then((r) => { if (live) setFact(r.fact); }).catch(() => {});
    return () => { live = false; };
  }, []);

  if (!fact || gone) return null;

  return (
    <div className="flex max-w-reading animate-fade items-start gap-3 border-l border-line pl-4 sm:pl-5">
      <p className="min-w-0 flex-1 text-[.8125rem] leading-relaxed text-ink-3">
        <span className="font-mono text-[.625rem] uppercase tracking-[.12em]">
          last time · {dayLabel(fact.lastSeen).toLowerCase()}
        </span>
        <span className="mt-0.5 block text-ink-2">{fact.detail}</span>
      </p>
      <button onClick={() => setGone(true)} aria-label="Dismiss"
              className="shrink-0 rounded px-1 font-mono text-[.75rem] text-ink-3 hover:text-ink-2">
        ×
      </button>
    </div>
  );
}
