"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { ApiError, api } from "@/lib/api";

/**
 * Astralis asks what to call you.
 *
 * This is written as the agent's first turn rather than as a settings form,
 * because it is the first thing it says and it sets the register for
 * everything after. The advice against a real name is given with the reason
 * attached — people ignore instructions and follow explanations.
 */
const ADJECTIVES = ["quiet", "slow", "north", "amber", "still", "open", "late", "plain", "even", "wide"];
const NOUNS = ["harbour", "signal", "meadow", "lantern", "current", "thicket", "window", "compass", "drift", "ember"];

function suggest() {
  const a = ADJECTIVES[Math.floor(Math.random() * ADJECTIVES.length)];
  const n = NOUNS[Math.floor(Math.random() * NOUNS.length)];
  return `${a}${n}`;
}

const LINES = [
  "You are in. Before anything else, I need something to call you.",
  "Pick anything. It does not have to mean anything, and it does not have to be your name. I would gently suggest it is not: other people here will see it, and you will say more honestly to me if what you say is not attached to who you are at work.",
  "You can change it later.",
];

export default function Onboarding({ onDone }: { onDone: (handle: string) => void }) {
  const [shown, setShown] = useState(0);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const input = useRef<HTMLInputElement>(null);
  const options = useMemo(() => Array.from({ length: 3 }, suggest), []);

  // Reveal the lines in sequence. It reads as someone speaking rather than a
  // wall of terms appearing at once.
  useEffect(() => {
    if (shown >= LINES.length) return;
    const t = setTimeout(() => setShown((n) => n + 1), shown === 0 ? 220 : 620);
    return () => clearTimeout(t);
  }, [shown]);

  useEffect(() => {
    if (shown >= LINES.length) input.current?.focus();
  }, [shown]);

  async function submit() {
    const handle = value.trim();
    if (handle.length < 3 || busy) return;
    setBusy(true); setError(null);
    try {
      await api("/me/handle", { method: "POST", body: JSON.stringify({ handle }) });
      onDone(handle);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Something went wrong.");
      setBusy(false);
    }
  }

  const ready = shown >= LINES.length;

  return (
    <div data-butterfly-clear="content" className="mx-auto flex h-full max-w-2xl flex-col justify-center px-6 py-10">
      <p className="label mb-6">Astralis</p>

      <div className="space-y-4 border-l border-violet/40 pl-5 sm:pl-6">
        {LINES.slice(0, shown).map((l, i) => (
          <p key={i} className="reply max-w-reading animate-rise">{l}</p>
        ))}
        {!ready && (
          <span className="block h-4 w-2 animate-caret bg-violet/70" aria-hidden />
        )}
      </div>

      {ready && (
        <div className="mt-9 animate-rise">
          <label htmlFor="handle" className="label">Call me</label>

          <div className="mt-2.5 flex items-center gap-2 rounded-xl border border-line bg-surface
                          px-3.5 py-3 transition-colors focus-within:border-violet/60">
            <span className="caret select-none text-[.9375rem]" aria-hidden>›</span>
            <input
              id="handle"
              ref={input}
              value={value}
              onChange={(e) => setValue(e.target.value.slice(0, 24))}
              onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); void submit(); } }}
              placeholder="anything but your real name"
              autoComplete="off"
              spellCheck={false}
              className="min-w-0 flex-1 bg-transparent font-mono text-[.9375rem] text-ink
                         outline-none placeholder:text-ink-3/70"
            />
            <button className="btn-primary !px-3 !py-1.5" onClick={() => void submit()}
                    disabled={busy || value.trim().length < 3}>
              {busy ? "…" : "Continue"}
            </button>
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <span className="label">Or try</span>
            {options.map((o) => (
              <button key={o} onClick={() => { setValue(o); input.current?.focus(); }}
                      className="tag transition-colors hover:border-violet/50 hover:text-ink-2">
                {o}
              </button>
            ))}
          </div>

          {error && <p role="alert" className="mt-4 text-[.8125rem] text-amber">{error}</p>}

          <p className="mt-6 max-w-measure font-mono text-[.6875rem] leading-relaxed text-ink-3">
            3 to 24 characters. Letters, numbers, spaces, hyphens, underscores.
            Your email is never shown to anyone. This name is all anyone sees.
          </p>
        </div>
      )}
    </div>
  );
}
