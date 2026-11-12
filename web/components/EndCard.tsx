"use client";

import { useState } from "react";

import { rateSession } from "@/lib/api";

/**
 * one deliberate question per conversation instead of a thumb under every
 * reply. it is the cleanest signal the variant bandit gets: a whole
 * conversation, judged once, by the person who had it.
 */
export default function EndCard({ sessionId, earlier, onDone }: {
  sessionId: string;
  /** true when asked about the conversation they just left, not this one */
  earlier?: boolean;
  onDone: () => void;
}) {
  const [answered, setAnswered] = useState(false);

  async function rate(worth: boolean) {
    setAnswered(true); // optimistic, like feedback: never worth interrupting anyone
    try { await rateSession(sessionId, worth); } catch { /* non-critical */ }
    setTimeout(onDone, 1600);
  }

  return (
    <div className="card max-w-reading animate-fade p-4">
      {answered ? (
        <p className="font-mono text-[.625rem] leading-relaxed text-ink-3">
          thank you. this shapes how astralis talks, not just what it looks up
        </p>
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          <span className="mr-1 text-[.8125rem] text-ink-2">
            {earlier ? "Was that last conversation worth having?" : "Was this conversation worth having?"}
          </span>
          {([["Yes", true], ["Not really", false]] as const).map(([label, value]) => (
            <button key={label} onClick={() => void rate(value)}
                    className="tag font-sans !text-[.6875rem] !normal-case !tracking-normal text-ink-2
                               transition-colors hover:border-violet/50 hover:bg-raised hover:text-ink">
              {label}
            </button>
          ))}
          <button onClick={onDone} aria-label="Dismiss"
                  className="ml-auto rounded px-1 font-mono text-[.75rem] text-ink-3 hover:text-ink-2">
            ×
          </button>
        </div>
      )}
    </div>
  );
}
