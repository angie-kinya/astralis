"use client";

import { useState } from "react";

import type { Invite } from "@/lib/api";

/** four stars and two lines: the game's small sign, drawn in currentColor */
export function ConstellationGlyph({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className}
         fill="none" stroke="currentColor" strokeWidth={1.3} strokeLinecap="round">
      <path d="M6 7h12M18 7v10" opacity=".75" />
      <path d="M6 7v10M6 17h12" strokeDasharray=".01 2.4" opacity=".55" />
      {[[6, 7], [18, 7], [6, 17], [18, 17]].map(([x, y]) => (
        <circle key={`${x}${y}`} cx={x} cy={y} r={1.5} fill="currentColor" stroke="none" />
      ))}
    </svg>
  );
}

/**
 * an invite, at the end of the thread it belongs to. the recipient can accept
 * or let it go; the sender can take it back. nobody is told why an invite
 * lapsed, and a declined one simply disappears
 */
export default function InviteCard({ invite, onAccept, onDecline }: {
  invite: Invite;
  onAccept: () => Promise<void>;
  onDecline: () => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const mins = Math.max(1, Math.ceil((new Date(invite.expiresAt).getTime() - Date.now()) / 60_000));

  async function run(f: () => Promise<void>) {
    setBusy(true);
    try { await f(); } finally { setBusy(false); }
  }

  return (
    <div className={`flex animate-rise ${invite.mine ? "justify-end" : ""}`}>
      <div className="bubble max-w-[85%] border-violet/25 bg-surface/80">
        <p className="flex items-center gap-2 font-mono text-[.625rem] uppercase tracking-[.14em] text-violet-2">
          <ConstellationGlyph className="h-4 w-4" />
          constellations
        </p>
        <p className="mt-2 font-sans text-[.875rem] leading-relaxed text-ink">
          {invite.mine
            ? `You asked ${invite.peerHandle} to join some stars with you.`
            : `${invite.peerHandle} would like to join some stars with you. A few quiet minutes, no clock.`}
        </p>
        <p className="mt-1 font-mono text-[.5625rem] text-ink-3">
          {invite.mine ? `waits for them for about ${mins} min` : `open for about ${mins} min`}
        </p>
        <div className="mt-3 flex flex-wrap gap-2">
          {invite.mine ? (
            <button className="btn-quiet !px-3 !py-1.5" disabled={busy} onClick={() => void run(onDecline)}>
              Cancel invite
            </button>
          ) : (
            <>
              <button className="btn-primary !px-3 !py-1.5" disabled={busy} onClick={() => void run(onAccept)}>
                Accept
              </button>
              <button className="btn-quiet !px-3 !py-1.5" disabled={busy} onClick={() => void run(onDecline)}>
                Not now
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
