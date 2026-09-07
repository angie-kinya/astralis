"use client";

import { useEffect, useState } from "react";

import { api, type Me } from "@/lib/api";

/**
 * The right-to-erasure screen.
 *
 * Deliberately not hidden behind three menus. Kenya's Data Protection Act 2019
 * gives data subjects a right to erasure; a right nobody can find is not a
 * right. The type-to-confirm exists because this is genuinely irreversible —
 * ON DELETE CASCADE takes the sessions, messages, summaries, ratings and
 * crisis flags with the user row.
 */
export default function SettingsPage() {
  const [me, setMe] = useState<Me | null>(null);
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => { api<Me>("/me").then(setMe).catch((e) => setError(e.message)); }, []);

  async function destroy() {
    setBusy(true); setError(null);
    try {
      await api("/me", { method: "DELETE" });
      window.location.href = "/api/auth/signout";
    } catch (e) {
      setError(e instanceof Error ? e.message : "Deletion failed.");
      setBusy(false);
    }
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-2xl px-5 py-10 sm:px-6">
        <p className="eyebrow">Your data</p>
        <h1 className="mt-3 font-serif text-[2rem] leading-tight tracking-[-0.02em]">
          What Astralis holds about you
        </h1>

        <dl className="mt-8 divide-y divide-line rounded-xl border border-line bg-surface">
          {[
            ["Account", me ? `${me.displayName || "—"} · ${me.email}` : "…"],
            ["Consent", me?.consented ? `Given · version ${me.consentVersion}` : "Not given"],
            ["Messages", "Every message you send and every reply, kept until you delete them"],
            ["Summaries", "One short factual note per conversation, used as memory"],
            ["Ratings", "Your helpful / not-helpful marks, used to reweight retrieval"],
          ].map(([k, v]) => (
            <div key={k} className="flex flex-col gap-1 px-5 py-4 sm:flex-row sm:gap-6">
              <dt className="w-32 shrink-0 font-mono text-[0.6875rem] uppercase
                             tracking-[0.14em] text-ink-3">{k}</dt>
              <dd className="text-[0.875rem] leading-relaxed text-ink-2">{v}</dd>
            </div>
          ))}
        </dl>

        <section className="mt-10 rounded-xl border border-signal/30 bg-signal/[0.05] p-5 sm:p-6">
          <h2 className="font-serif text-[1.1875rem] text-ink">Delete everything</h2>
          <p className="mt-2 max-w-measure text-[0.875rem] leading-relaxed text-ink-2">
            This permanently removes your account, every conversation, every summary and
            every rating. It cannot be undone and there is no backup you can be restored
            from. Research passages stay — they were never yours, and they contain nothing
            about you.
          </p>

          <label className="mt-5 block">
            <span className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-ink-3">
              Type DELETE to confirm
            </span>
            <input
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              autoComplete="off"
              spellCheck={false}
              className="mt-2 w-48 rounded-lg border border-line bg-ground px-3 py-2
                         font-mono text-[0.8125rem] text-ink outline-none
                         focus:border-line-strong"
            />
          </label>

          {error && <p className="mt-4 text-[0.8125rem] text-signal">{error}</p>}

          <button
            className="btn-danger mt-5"
            disabled={confirm !== "DELETE" || busy}
            onClick={() => void destroy()}
          >
            {busy ? "Deleting…" : "Delete my account and all my data"}
          </button>
        </section>
      </div>
    </div>
  );
}
