"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { catalog, createGame, userMessage } from "@/lib/api";

type Choice = { id: string; name: string; blurb: string };

const KEY = "astralis.words.pick";

// the last topic and mode, kept per browser as a convenience; storage can be absent or throw
function remembered(): { topic?: string; mode?: string } {
  try { return JSON.parse(window.localStorage.getItem(KEY) ?? "{}") as { topic?: string; mode?: string }; }
  catch { return {}; }
}
function remember(v: { topic: string; mode: string }) {
  try { window.localStorage.setItem(KEY, JSON.stringify(v)); } catch { /* fine without it */ }
}

/**
 * pick a topic and a way to play, then today's puzzle or another one.
 * mystery hides the topic until the end, so the topic picker steps aside for it
 */
export default function WordsStart() {
  const router = useRouter();
  const [topics, setTopics] = useState<Choice[] | null>(null);
  const [modes, setModes] = useState<Choice[]>([]);
  const [topic, setTopic] = useState("");
  const [mode, setMode] = useState("classic");
  const [busy, setBusy] = useState<"daily" | "practice" | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    catalog()
      .then((c) => {
        if (!live) return;
        const t = c.words.topics, m = c.words.modes;
        const last = remembered();
        setTopics(t); setModes(m);
        setTopic(t.find((x) => x.id === last.topic)?.id ?? t[0]?.id ?? "");
        setMode(m.find((x) => x.id === last.mode)?.id ?? m.find((x) => x.id === "classic")?.id ?? m[0]?.id ?? "classic");
      })
      .catch((e) => { if (live) setError(userMessage(e)); });
    return () => { live = false; };
  }, []);

  const mystery = mode === "mystery";

  async function start(daily: boolean) {
    setBusy(daily ? "daily" : "practice"); setError(null);
    remember({ topic, mode });
    try {
      const options: { mode: string; topic?: string; daily?: boolean } = { mode };
      if (!mystery && topic) options.topic = topic;
      if (daily) options.daily = true;
      const { game } = await createGame("words", "solo", options);
      router.push(`/play/${game.id}`);
    } catch (e) {
      setError(userMessage(e));
      setBusy(null);
    }
  }

  if (!topics) {
    return error
      ? <p role="alert" className="font-mono text-[.75rem] text-amber">{error}</p>
      : <p className="font-mono text-[.75rem] text-ink-3">laying out the tiles…</p>;
  }

  return (
    <div className="space-y-5">
      <Picker label="how to play" value={mode} onChange={setMode} options={modes} cols="sm:grid-cols-3" />

      {mystery ? (
        <p className="rounded-xl border border-dashed border-line px-3.5 py-3 font-mono text-[.6875rem] leading-relaxed text-ink-3">
          the topic stays hidden until the end. part of the fun is working out where the word lives.
        </p>
      ) : (
        <Picker label="topic" value={topic} onChange={setTopic} options={topics} cols="sm:grid-cols-2" accent={TOPIC_ACCENT} />
      )}

      <div className="flex flex-wrap gap-2">
        <button type="button" className="btn-primary" disabled={!!busy || (!mystery && !topic)} onClick={() => void start(true)}>
          {busy === "daily" ? "Opening…" : "Today's puzzle"}
        </button>
        <button type="button" className="btn-quiet" disabled={!!busy || (!mystery && !topic)} onClick={() => void start(false)}>
          {busy === "practice" ? "Finding one…" : "Another one"}
        </button>
      </div>
      <p className="font-mono text-[.625rem] leading-relaxed text-ink-3">
        today&apos;s puzzle is the same for everyone and waits for you if you leave. another one is fresh every time.
      </p>

      {error && <p role="alert" className="font-mono text-[.75rem] text-amber">{error}</p>}
    </div>
  );
}

// a small dot per topic so the cards are told apart at a glance. topics the
// client does not know yet simply go without
const TOPIC_ACCENT: Record<string, string> = {
  calm: "--g-teal", astronomy: "--violet-2", mythology: "--amber", tech: "--g-sky", anime: "--p-bot",
};

/** a radio group drawn as cards: arrows move between them, as a radio group should */
function Picker({ label, value, onChange, options, cols, accent }: {
  label: string; value: string; onChange: (v: string) => void; options: Choice[]; cols: string;
  accent?: Record<string, string>;
}) {
  function onKeyDown(e: React.KeyboardEvent) {
    const i = options.findIndex((o) => o.id === value);
    const step = e.key === "ArrowRight" || e.key === "ArrowDown" ? 1 : e.key === "ArrowLeft" || e.key === "ArrowUp" ? -1 : 0;
    if (!step || i < 0) return;
    e.preventDefault();
    const next = options[(i + step + options.length) % options.length]!;
    onChange(next.id);
    (e.currentTarget.querySelector(`[data-id="${next.id}"]`) as HTMLElement | null)?.focus();
  }

  return (
    <fieldset>
      <legend className="label mb-2">{label}</legend>
      <div role="radiogroup" aria-label={label} onKeyDown={onKeyDown} className={`grid grid-cols-1 gap-2 ${cols}`}>
        {options.map((o) => {
          const on = o.id === value;
          return (
            <button key={o.id} type="button" role="radio" aria-checked={on} data-id={o.id}
                    tabIndex={on ? 0 : -1} onClick={() => onChange(o.id)}
                    className={`rounded-xl border px-3.5 py-2.5 text-left transition-colors duration-150
                                ${on ? "border-violet/60 bg-violet/10" : "border-line bg-surface/60 hover:border-line-2 hover:bg-raised/60"}`}>
              <span className={`flex items-center gap-2 font-mono text-[.8125rem] ${on ? "text-ink" : "text-ink-2"}`}>
                {accent?.[o.id] && (
                  <span aria-hidden className="h-1.5 w-1.5 shrink-0 rounded-full"
                        style={{ background: `rgb(var(${accent[o.id]}))` }} />
                )}
                {o.name}
              </span>
              {o.blurb && <span className="mt-0.5 block text-[.75rem] leading-snug text-ink-3">{o.blurb}</span>}
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}
