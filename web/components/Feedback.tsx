"use client";

import { useState } from "react";

import { api } from "@/lib/api";

/**
 * The single signal that makes the system improve.
 *
 * The acknowledgement text names the mechanism on purpose — "this changes what
 * I look up next time" is true, and telling people the truth about what their
 * input does is what makes them willing to give it.
 */
export default function Feedback({ recommendationId }: { recommendationId: string }) {
  const [rated, setRated] = useState<1 | -1 | null>(null);

  async function rate(rating: 1 | -1) {
    if (rated) return;
    setRated(rating); // optimistic: a failed rating is not worth interrupting anyone
    try {
      await api("/feedback", {
        method: "POST",
        body: JSON.stringify({ recommendationId, rating }),
      });
    } catch { /* non-critical by design */ }
  }

  if (rated) {
    return (
      <p className="mt-5 font-mono text-[0.6875rem] leading-relaxed text-ink-3">
        {rated === 1
          ? "Noted — the research behind this is weighted up for next time."
          : "Noted — weighted down. It will surface less often."}
      </p>
    );
  }

  return (
    <div className="mt-5 flex items-center gap-2">
      <span className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-ink-3">
        Did this help?
      </span>
      {([["Yes", 1], ["Not really", -1]] as const).map(([label, value]) => (
        <button
          key={label}
          onClick={() => void rate(value)}
          className="rounded-md border border-line px-2.5 py-1 text-[0.75rem] text-ink-2
                     transition-colors hover:border-accent/50 hover:bg-raised hover:text-ink"
        >
          {label}
        </button>
      ))}
    </div>
  );
}
