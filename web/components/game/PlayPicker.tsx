"use client";

import { useEffect, useRef, useState } from "react";

import { ConstellationGlyph, WordsGlyph } from "@/components/game/Glyphs";
import LevelPicker from "@/components/game/LevelPicker";
import { useCatalog, useProgress } from "@/components/game/useCatalog";
import type { GameOptions, Invite } from "@/lib/api";

/**
 * what "play" in a chat opens: pick a game and how to play it, then one
 * invite goes into the thread. constellations offers the skies you have open
 * (the server checks the other person has it too); a words duel picks a topic
 * and a mode. esc or the button again closes it without sending anything
 */
export default function PlayPicker({ handle, onInvite, onClose }: {
  handle: string;
  onInvite: (kind: Invite["kind"], options: GameOptions) => Promise<void>;
  onClose: () => void;
}) {
  const cat = useCatalog();
  const { unlocked } = useProgress();
  const [kind, setKind] = useState<Invite["kind"]>("constellations");
  const [level, setLevel] = useState<number | null>(null);
  const [topic, setTopic] = useState<string | null>(null);
  const [mode, setMode] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => { if (unlocked !== null && level === null) setLevel(Math.min(unlocked, 2)); }, [unlocked, level]);
  useEffect(() => {
    if (!cat) return;
    setTopic((t) => t ?? cat.words.topics[0]?.id ?? null);
    setMode((m) => m ?? cat.words.modes[0]?.id ?? null);
  }, [cat]);

  useEffect(() => { box.current?.querySelector<HTMLElement>("[data-autofocus]")?.focus(); }, []);

  async function send() {
    setBusy(true);
    try {
      await onInvite(kind, kind === "words" ? { topic: topic ?? undefined, mode: mode ?? undefined } : { level: level ?? 2 });
    } finally { setBusy(false); }
  }

  const ready = kind === "words" ? !!topic && !!mode : level !== null;

  return (
    <div ref={box} role="dialog" aria-label={`play with ${handle}`}
         onKeyDown={(e) => { if (e.key === "Escape") { e.stopPropagation(); onClose(); } }}
         className="absolute inset-x-3 top-full z-20 mt-1 max-h-[70dvh] overflow-y-auto rounded-xl border border-line
                    bg-surface p-3 shadow-xl sm:left-auto sm:w-[22rem]">
      <div role="tablist" aria-label="game" className="grid grid-cols-2 gap-1 rounded-lg bg-raised/60 p-1">
        {([["constellations", ConstellationGlyph], ["words", WordsGlyph]] as const).map(([k, G]) => (
          <button key={k} role="tab" aria-selected={kind === k} data-autofocus={kind === k ? "" : undefined}
                  onClick={() => setKind(k)}
                  className={`flex min-h-9 items-center justify-center gap-1.5 rounded-md font-mono text-[.6875rem]
                              transition-colors duration-150
                              ${kind === k ? "bg-surface text-ink shadow-sm" : "text-ink-3 hover:text-ink-2"}`}>
            <G className="h-3.5 w-3.5" />
            {k === "words" ? "words duel" : "constellations"}
          </button>
        ))}
      </div>

      <div className="mt-3">
        {kind === "constellations" ? (
          unlocked === null
            ? <div className="skel h-2 w-32" />
            : <LevelPicker levels={cat?.constellations.levels} unlocked={unlocked}
                           value={level ?? 1} onChange={setLevel} cap={Math.min(5, unlocked + 1)} />
        ) : !cat ? (
          <div className="skel h-2 w-32" />
        ) : (
          <div className="space-y-3">
            <Choice label="topic" items={cat.words.topics} value={topic} onChange={setTopic} />
            <Choice label="mode" items={cat.words.modes} value={mode} onChange={setMode} />
            <p className="font-mono text-[.5625rem] leading-relaxed text-ink-3">
              you each solve the same word on your own, then see how it went for both of you.
            </p>
          </div>
        )}
      </div>

      <div className="mt-3 flex justify-end gap-2">
        <button className="btn-quiet !px-3 !py-1.5" onClick={onClose}>Cancel</button>
        <button className="btn-primary !px-3 !py-1.5" disabled={busy || !ready} onClick={() => void send()}>
          {busy ? "Sending…" : `Invite ${handle}`}
        </button>
      </div>
    </div>
  );
}

function Choice({ label, items, value, onChange }: {
  label: string; items: { id: string; name: string; blurb: string }[]; value: string | null; onChange: (id: string) => void;
}) {
  const picked = items.find((i) => i.id === value);
  return (
    <div>
      <p className="label mb-1.5">{label}</p>
      <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-1.5">
        {items.map((i) => (
          <button key={i.id} role="radio" aria-checked={value === i.id} onClick={() => onChange(i.id)}
                  className={`min-h-8 rounded-lg border px-2.5 font-mono text-[.6875rem] transition-colors duration-150
                              ${value === i.id ? "border-violet/60 bg-violet/10 text-ink"
                                : "border-line text-ink-2 hover:border-line-2 hover:bg-raised/60"}`}>
            {i.name}
          </button>
        ))}
      </div>
      {picked?.blurb && <p className="mt-1 font-mono text-[.5625rem] text-ink-3">{picked.blurb}</p>}
    </div>
  );
}
