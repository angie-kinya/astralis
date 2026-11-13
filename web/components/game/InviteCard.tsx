"use client";

import { useState } from "react";

import { KindGlyph } from "@/components/game/Glyphs";
import { LEVEL_NAMES } from "@/components/game/rules";
import type { Invite } from "@/lib/api";

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

  const kind = invite.kind ?? "constellations";
  const o = invite.options ?? {};
  const detail = kind === "words"
    ? [o.topic, o.mode].filter(Boolean).join(", ")
    : o.level ? LEVEL_NAMES[o.level] ?? "" : "";

  function copy(): string {
    if (kind === "words") {
      return invite.mine
        ? `You asked ${invite.peerHandle} for a words duel. You each solve the same word on your own.`
        : `${invite.peerHandle} would like a words duel. You each solve the same word on your own, then compare.`;
    }
    return invite.mine
      ? `You asked ${invite.peerHandle} to join some stars with you.`
      : `${invite.peerHandle} would like to join some stars with you. A few quiet minutes, no clock.`;
  }

  async function run(f: () => Promise<void>) {
    setBusy(true);
    try { await f(); } finally { setBusy(false); }
  }

  return (
    <div className={`flex animate-rise ${invite.mine ? "justify-end" : ""}`}>
      <div className="bubble max-w-[85%] border-violet/25 bg-surface/80">
        <p className="flex items-center gap-2 font-mono text-[.625rem] uppercase tracking-[.14em] text-violet-2">
          <KindGlyph kind={kind} className="h-4 w-4" />
          {kind === "words" ? "words duel" : "constellations"}
          {detail && <span className="normal-case tracking-normal text-ink-3">· {detail}</span>}
        </p>
        <p className="mt-2 font-sans text-[.875rem] leading-relaxed text-ink">{copy()}</p>
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
