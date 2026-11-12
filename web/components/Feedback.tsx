"use client";

import { useState } from "react";

import { api } from "@/lib/api";

/**
 * The single signal that makes the system improve.
 *
 * The acknowledgement names the mechanism on purpose — "this changes what I
 * look up next time" is true, and telling people the truth about what their
 * input does is what makes them willing to give it.
 *
 * hidden until hover, keyboard focus or a long-press (`revealed`). a survey
 * under every message is a product tell and causes rating fatigue; fewer,
 * deliberate ratings are better data for retrieval and the variant bandit.
 */
export default function Feedback({ recommendationId, revealed = false }: {
  recommendationId: string;
  revealed?: boolean;
}) {
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
      <p className="mt-2 font-mono text-[.625rem] leading-relaxed text-ink-3 animate-fade">
        {rated === 1
          ? "noted. the research behind this is weighted up for next time"
          : "noted. it is weighted down and will surface less often"}
      </p>
    );
  }

  return (
    <div className={`mt-1 flex flex-wrap items-center gap-2 transition-opacity duration-200 ease-swift
                     focus-within:opacity-100 group-hover:opacity-100
                     ${revealed ? "opacity-100" : "opacity-0"}`}>
      <span className="label">Did this help?</span>
      {([["Yes", 1], ["Not really", -1]] as const).map(([label, value]) => (
        <button key={label} onClick={() => void rate(value)}
                className="tag font-sans !text-[.6875rem] !normal-case !tracking-normal text-ink-2
                           transition-colors hover:border-violet/50 hover:bg-raised hover:text-ink">
          {label}
        </button>
      ))}
    </div>
  );
}
