"use client";

import { useState } from "react";

import { api, signOut } from "@/lib/api";

/**
 * A participant information sheet, not a cookie banner.
 *
 * Two design decisions carry real weight:
 *  1. The consent button stays disabled until the sheet has been scrolled to
 *     the end. Consent that was never read is not consent, and Kenya's Data
 *     Protection Act expects it to be informed and specific.
 *  2. Every claim here is one the system actually keeps. "You can delete
 *     everything" is backed by ON DELETE CASCADE, not by good intentions.
 *
 * The API enforces the same gate independently — this screen is courtesy, the
 * requireConsent middleware is the boundary.
 */
const SECTIONS = [
  {
    h: "What Astralis is",
    p: "A personal research project built by a student. It is a reflection and journalling companion for work stress and burnout. It is not therapy, not a diagnosis, not a clinical tool, and not a crisis service. Nothing it says should be treated as medical advice.",
  },
  {
    h: "What is stored",
    p: "Your email address (to sign you in), the name you chose, every message you send and every reply you receive, and a short factual summary of each conversation so the tool has continuity between sessions. Nothing else.",
  },
  {
    h: "Your email is never visible to anyone",
    p: "Other people using Astralis see only the name you chose. Your email is not shown in the circle, beside your messages, or anywhere another person can reach, and it is never linked to your name for anyone but you. It is used to send sign-in codes and \u2014 only if you asked \u2014 release notes. It is never shared, sold, or used for marketing. Sign-in emails are delivered through Resend, an email provider, which sees the address only to deliver them.",
  },
  {
    h: "Who else sees it",
    p: "Your messages are sent to Anthropic's API to generate replies, under their commercial terms. Retrieved research passages are added to that request. Nobody else has access to your conversations, and they are never published.",
  },
  {
    h: "How it uses your feedback",
    p: "When you mark a reply helpful, the research passages behind it are weighted slightly up in future searches — for everyone, not just you. Your words are never used to train or fine-tune any model.",
  },
  {
    h: "If a message suggests crisis",
    p: "A filter runs before the model sees your message. If it detects language suggesting self-harm or crisis, the model is never called: you get a fixed response with real helplines, and that message is excluded from the conversation memory. A flag is recorded so the filter can be checked for accuracy.",
  },
  {
    h: "Talking to other people",
    p: "You can opt in to a circle of other people using Astralis. If you do, they see only the name you chose \u2014 never your email, which is not linked to your name for anyone. Messages between you are stored so they can be delivered when someone is offline. Peer support is not crisis support, and the same safety filter applies to those messages too. You can leave at any time.",
  },
  {
    h: "Your notes",
    p: "The notepad is private in a specific technical sense: nothing written there is embedded, retrieved, summarised, or sent to any model. It is stored so you can come back to it, and it is deleted with everything else if you ask.",
  },
  {
    h: "Your rights",
    p: "You can delete your account and every message, summary, and rating permanently, at any time, from the 'Your data' page. Deletion is immediate and irreversible. You can stop using Astralis at any point without giving a reason.",
  },
  {
    h: "Research use",
    p: "Aggregate, de-identified patterns may inform academic work. Your individual conversations will not be. If this data is ever proposed for formal research, that requires a separate ethics review and a fresh, specific request for your consent — this agreement does not cover it.",
  },
];

export default function Consent({ version, onAccepted }: {
  version: string;
  onAccepted: () => void;
}) {
  const [read, setRead] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function accept() {
    setBusy(true); setError(null);
    try {
      await api("/consent", { method: "POST" });
      onAccepted();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong.");
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto flex h-full max-w-2xl flex-col px-5 py-8 sm:px-6">
      <header className="shrink-0">
        <p className="eyebrow">Participation terms · version {version}</p>
        <h1 className="mt-3 font-serif text-[2rem] leading-tight tracking-[-0.02em]">
          Before we begin
        </h1>
        <p className="mt-3 max-w-measure text-[0.9375rem] leading-relaxed text-ink-2">
          Please read this in full. It is short, and everything in it is enforced
          by the system rather than promised by it.
        </p>
      </header>

      <div
        onScroll={(e) => {
          const el = e.currentTarget;
          if (el.scrollTop + el.clientHeight >= el.scrollHeight - 24) setRead(true);
        }}
        className="my-7 min-h-0 flex-1 overflow-y-auto rounded-xl border border-line
                   bg-surface p-5 sm:p-6"
      >
        <ol className="space-y-6">
          {SECTIONS.map((s, i) => (
            <li key={s.h} className="flex gap-4">
              <span className="mt-1 font-mono text-[0.6875rem] tabular-nums text-accent">
                {String(i + 1).padStart(2, "0")}
              </span>
              <div>
                <h2 className="font-serif text-[1.0625rem] text-ink">{s.h}</h2>
                <p className="mt-1.5 text-[0.875rem] leading-[1.68] text-ink-2">{s.p}</p>
              </div>
            </li>
          ))}
        </ol>
        <p className="mt-8 border-t border-line pt-5 font-mono text-[0.6875rem] leading-relaxed text-ink-3">
          Questions about any of this are welcome and expected. Ask before you agree,
          not after.
        </p>
      </div>

      {error && <p className="mb-4 text-[0.8125rem] text-signal">{error}</p>}

      <div className="flex shrink-0 flex-wrap items-center gap-3">
        <button className="btn-primary" onClick={() => void accept()} disabled={!read || busy}>
          {busy ? "Recording…" : "I have read this — continue"}
        </button>
        <button className="btn-quiet" onClick={() => void signOut()}>Not now</button>
        {!read && (
          <span className="font-mono text-[0.6875rem] text-ink-3">
            Scroll to the end to continue
          </span>
        )}
      </div>
    </div>
  );
}
