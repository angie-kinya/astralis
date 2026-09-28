"use client";

import { useState } from "react";

import { useMe } from "@/components/AppGate";
import { ApiError, api, signOut } from "@/lib/api";

/**
 * Identity, preferences and erasure in one place.
 *
 * Deliberately not buried behind three menus. Kenya's Data Protection Act 2019
 * gives data subjects a right to erasure, and a right nobody can find is not a
 * right. Type-to-confirm exists because deletion is genuinely irreversible:
 * ON DELETE CASCADE takes sessions, messages, summaries, ratings, notes, peer
 * messages, sign-in sessions and crisis flags with the user row.
 */
export default function SettingsPage() {
  const { me, setMe } = useMe();
  const [handle, setHandle] = useState(me.handle);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function rename() {
    const h = handle.trim();
    if (h === me.handle || h.length < 3) return;
    setSaving(true); setError(null); setSaved(false);
    try {
      await api("/me/handle", { method: "POST", body: JSON.stringify({ handle: h }) });
      setMe({ ...me, handle: h });
      setSaved(true);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not save that name.");
    } finally { setSaving(false); }
  }

  async function toggle(path: string, field: "peerOptIn" | "notifyReleases", value: boolean) {
    try {
      await api(path, { method: "POST", body: JSON.stringify({ [field]: value }) });
      setMe({ ...me, [field]: value });
    } catch (e) { setError(e instanceof Error ? e.message : "Could not update that."); }
  }

  async function destroy() {
    setBusy(true); setError(null);
    try {
      await api("/me", { method: "DELETE" });
      window.location.assign("/");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Deletion failed.");
      setBusy(false);
    }
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-2xl px-5 py-7 sm:px-8">
        <p className="label">You</p>
        <h1 className="mt-3 font-sans text-[1.75rem] font-semibold leading-tight tracking-[-.025em]">
          Your name, your email, your data
        </h1>

        {/* ------------------------------------------------------- identity */}
        <section className="card mt-7 p-5">
          <label htmlFor="handle" className="label">What Astralis calls you</label>
          <div className="mt-2.5 flex flex-wrap items-center gap-2">
            <div className="flex min-w-[12rem] flex-1 items-center gap-2 rounded-lg border
                            border-line bg-bg px-3 py-2 focus-within:border-violet/60">
              <span className="caret select-none text-[.875rem]" aria-hidden>›</span>
              <input id="handle" value={handle} spellCheck={false} autoComplete="off"
                     onChange={(e) => { setHandle(e.target.value.slice(0, 24)); setSaved(false); }}
                     className="min-w-0 flex-1 bg-transparent font-mono text-[.875rem] text-ink outline-none" />
            </div>
            <button className="btn-quiet" onClick={() => void rename()}
                    disabled={saving || handle.trim() === me.handle || handle.trim().length < 3}>
              {saving ? "Saving…" : saved ? "Saved" : "Rename"}
            </button>
          </div>
          <p className="mt-2.5 font-mono text-[.625rem] leading-relaxed text-ink-3">
            the only thing other people ever see.
          </p>

          <div className="hair my-5" />

          <p className="label">Your email</p>
          <p className="mt-2 font-mono text-[.875rem] text-ink">{me.email}</p>
          <p className="mt-1.5 font-mono text-[.625rem] leading-relaxed text-ink-3">
            only you can see this. it is never shown to anyone else or linked to your name for anyone.
          </p>
        </section>

        {/* ---------------------------------------------------- preferences */}
        <Toggle
          title="Release notes"
          on={me.notifyReleases}
          onText="You get a short email when a new version is released. Nothing else is ever sent."
          offText="Off. You will only ever get sign-in codes."
          onClick={() => void toggle("/me/notify", "notifyReleases", !me.notifyReleases)}
          labels={["Turn on", "Turn off"]}
        />
        <Toggle
          title="The circle"
          on={me.peerOptIn}
          onText="You appear in other people's lists by your chosen name, and can be written to."
          offText="You are not listed, and nobody can write to you."
          onClick={() => void toggle("/me/peer-opt-in", "peerOptIn", !me.peerOptIn)}
          labels={["Join", "Leave"]}
        />

        {/* ----------------------------------------------------------- data */}
        <h2 className="label mt-10">What Astralis holds</h2>
        <dl className="mt-3 divide-y divide-line rounded-xl border border-line bg-surface">
          {[
            ["Email", "To sign you in, and release notes if you asked. Visible only to you"],
            ["Consent", me.consented ? `Given · version ${me.consentVersion}` : "Not given"],
            ["Conversations", "Every message to Astralis and every reply"],
            ["Memory", "One short factual note per session, used as context"],
            ["Ratings", "Your helpful / not-helpful marks, which reweight retrieval"],
            ["Circle", "Messages between you and other people here"],
            ["Notes", "Your notepad. Never read by any model"],
          ].map(([k, v]) => (
            <div key={k} className="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:gap-5">
              <dt className="label w-28 shrink-0 pt-0.5">{k}</dt>
              <dd className="text-[.8125rem] leading-relaxed text-ink-2">{v}</dd>
            </div>
          ))}
        </dl>

        {/* -------------------------------------------------------- erasure */}
        <section className="mt-6 rounded-xl border border-amber/30 bg-amber/[.05] p-5">
          <h2 className="font-sans text-[1rem] font-medium">Delete everything</h2>
          <p className="mt-2 max-w-measure text-[.8125rem] leading-relaxed text-ink-2">
            Permanently removes your account, your email address, and every row above. It
            cannot be undone and there is no backup you can be restored from. Research
            passages stay — they were never yours and contain nothing about you.
          </p>
          <label className="mt-4 block">
            <span className="label">Type DELETE to confirm</span>
            <input value={confirm} onChange={(e) => setConfirm(e.target.value)}
                   autoComplete="off" spellCheck={false}
                   className="mt-2 block w-44 rounded-lg border border-line bg-bg px-3 py-2
                              font-mono text-[.8125rem] text-ink outline-none focus:border-amber/60" />
          </label>
          <button className="btn-danger mt-4" disabled={confirm !== "DELETE" || busy}
                  onClick={() => void destroy()}>
            {busy ? "Deleting…" : "Delete my account and all my data"}
          </button>
        </section>

        {error && <p role="alert" className="mt-5 font-mono text-[.75rem] text-amber">{error}</p>}

        <div className="mt-10 flex items-center justify-between border-t border-line pt-5">
          <button onClick={() => void signOut()}
                  className="font-mono text-[.6875rem] text-ink-3 hover:text-ink-2">
            sign out
          </button>
          <span className="font-mono text-[.625rem] text-ink-3">astralis · closed beta</span>
        </div>
      </div>
    </div>
  );
}

function Toggle({ title, on, onText, offText, onClick, labels }: {
  title: string; on: boolean; onText: string; offText: string;
  onClick: () => void; labels: [string, string];
}) {
  return (
    <section className="card mt-4 flex items-start justify-between gap-4 p-5">
      <div>
        <p className="flex items-center gap-2 font-sans text-[.9375rem] font-medium">
          {title}
          <span className={`tag ${on ? "!border-cyan/40 !text-cyan" : ""}`}>{on ? "on" : "off"}</span>
        </p>
        <p className="mt-1.5 max-w-measure text-[.8125rem] leading-relaxed text-ink-2">
          {on ? onText : offText}
        </p>
      </div>
      <button className={on ? "btn-quiet" : "btn-primary"} onClick={onClick}>
        {on ? labels[1] : labels[0]}
      </button>
    </section>
  );
}
