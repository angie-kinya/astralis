"use client";

import { useEffect, useState } from "react";

import { deleteFact, listFacts, userMessage, type Fact } from "@/lib/api";
import { dayLabel } from "@/lib/format";

// the order a person would scan them in, with labels in their terms, not ours
const KINDS: [string, string][] = [
  ["person", "People"],
  ["event", "Things happening"],
  ["commitment", "Things you're trying"],
  ["helped", "What helped"],
  ["context", "Your work"],
];

/**
 * "what astralis remembers". every fact the agent can recall is listed here
 * and can be deleted, one at a time. memory that people can see is memory
 * they can trust; memory they cannot see is surveillance with good manners.
 *
 * a delete takes two clicks (the button asks once) and is then optimistic:
 * the row goes immediately and comes back only if the server refused.
 */
export default function Remembers() {
  const [facts, setFacts] = useState<Fact[] | null>(null);
  const [arming, setArming] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listFacts().then((r) => setFacts(r.facts ?? [])).catch((e) => {
      setFacts([]); setError(userMessage(e, "Could not load what Astralis remembers."));
    });
  }, []);

  async function remove(f: Fact) {
    setArming(null); setError(null);
    setFacts((prev) => prev?.filter((x) => x.id !== f.id) ?? null);
    try { await deleteFact(f.id); } catch (e) {
      setFacts((prev) => (prev ? [...prev, f].sort(byRecent) : prev));
      setError(userMessage(e, "Could not delete that. Try again."));
    }
  }

  return (
    <section className="mt-10">
      <h2 className="label">What Astralis remembers</h2>
      <p className="mt-2 max-w-measure text-[.8125rem] leading-relaxed text-ink-2">
        Short facts from your conversations: the people, events and things you&rsquo;re trying
        that you mentioned. Astralis brings one up only when it fits. Delete any of them and
        it is gone for good.
      </p>

      {facts === null ? (
        <div className="mt-4 space-y-2.5" aria-hidden>
          {[80, 64].map((w) => <div key={w} className="skel h-[.625rem]" style={{ width: `${w}%` }} />)}
        </div>
      ) : facts.length === 0 ? (
        <p className="mt-4 font-mono text-[.6875rem] text-ink-3">nothing yet.</p>
      ) : (
        <div className="mt-4 divide-y divide-line rounded-xl border border-line bg-surface">
          {KINDS.map(([kind, label]) => {
            const rows = facts.filter((f) => f.kind === kind);
            if (!rows.length) return null;
            return (
              <div key={kind} className="px-4 py-3">
                <p className="label pb-2">{label}</p>
                <ul className="space-y-2.5">
                  {rows.map((f) => (
                    <li key={f.id} className="flex items-start gap-3">
                      <div className="min-w-0 flex-1">
                        <p className="text-[.8125rem] leading-snug text-ink">
                          {f.subject}
                          {kind === "commitment" && f.status !== "open" && (
                            <span className="tag ml-2">{f.status}</span>
                          )}
                        </p>
                        <p className="mt-0.5 text-[.8125rem] leading-relaxed text-ink-2">{f.detail}</p>
                        <p className="mt-0.5 font-mono text-[.5625rem] text-ink-3">
                          {dayLabel(f.lastSeen).toLowerCase()}
                          {f.mentions > 1 && ` · mentioned ${f.mentions} times`}
                        </p>
                      </div>
                      <button onClick={() => (arming === f.id ? void remove(f) : setArming(f.id))}
                              onBlur={() => setArming((a) => (a === f.id ? null : a))}
                              className={`shrink-0 rounded font-mono text-[.625rem] transition-colors
                                          ${arming === f.id ? "text-amber" : "text-ink-3 hover:text-ink-2"}`}>
                        {arming === f.id ? "delete?" : "forget"}
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            );
          })}
        </div>
      )}

      {error && <p role="alert" className="mt-3 font-mono text-[.75rem] text-amber">{error}</p>}
    </section>
  );
}

const byRecent = (a: Fact, b: Fact) => b.lastSeen.localeCompare(a.lastSeen);
