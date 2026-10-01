"use client";

import { useEffect, useRef, useState } from "react";

import { ApiError, api } from "@/lib/api";

type Step = "email" | "code";

/**
 * Email sign-in, in two steps: an address, then a 6-digit code sent to it.
 *
 * The code is the point. Accepting a typed email on trust would mean anyone
 * who typed yours could read your conversations; proving you can open the
 * inbox is what makes an email an identity.
 *
 * There is no separate "sign up". A new address and a returning one go
 * through the same two steps, so the form never reveals whether a given email
 * has an account here — for a mental-health tool, that fact is itself private.
 */
export default function SignIn() {
  const [step, setStep] = useState<Step>("email");
  const [email, setEmail] = useState("");
  const [notify, setNotify] = useState(false);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);
  const [devCode, setDevCode] = useState<string | null>(null);
  const codeRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((n) => n - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  useEffect(() => { if (step === "code") codeRef.current?.focus(); }, [step]);

  async function sendCode(e?: React.FormEvent) {
    e?.preventDefault();
    if (busy || !email.trim()) return;
    setBusy(true); setError(null);
    try {
      const res = await api<{ sent: boolean; devCode?: string }>("/auth/start", {
        method: "POST", body: JSON.stringify({ email }),
      });
      setDevCode(res.devCode ?? null);
      setStep("code"); setCode(""); setCooldown(30);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not send a code. Check your connection.");
    } finally { setBusy(false); }
  }

  async function verify(value: string) {
    if (busy || value.length !== 6) return;
    setBusy(true); setError(null);
    try {
      await api("/auth/verify", {
        method: "POST",
        body: JSON.stringify({ email, code: value, notifyReleases: notify }),
      });
      // A full navigation, not a client push: the server layout needs to see
      // the cookie that was just set.
      window.location.assign("/chat");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not check that code.");
      setCode("");
      setBusy(false);
      codeRef.current?.focus();
    }
  }

  return (
    <div data-butterfly-clear="content" className="mx-auto flex min-h-full max-w-xl flex-col justify-center px-6 py-10">
      {step === "email" ? (
        <form onSubmit={sendCode} className="animate-rise">
          <p className="label">Sign in</p>
          <h1 className="mt-3 font-sans text-[1.875rem] font-semibold leading-tight tracking-[-.025em]">
            Start with your email.
          </h1>
          <p className="mt-3 max-w-measure text-[.9375rem] leading-[1.7] text-ink-2">
            We send a 6-digit code to it. No password to remember, and none to leak.
          </p>

          <label htmlFor="email" className="label mt-8 block">Email address</label>
          <div className="mt-2.5 flex items-center gap-2 rounded-xl border border-line bg-surface
                          px-3.5 py-3 transition-colors focus-within:border-violet/60">
            <span className="caret select-none text-[.9375rem]" aria-hidden>›</span>
            <input
              id="email" type="email" inputMode="email" autoComplete="email" autoFocus
              value={email} onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com" spellCheck={false} required
              className="min-w-0 flex-1 bg-transparent font-mono text-[.9375rem] text-ink
                         outline-none placeholder:text-ink-3/70"
            />
          </div>

          <label className="mt-5 flex cursor-pointer items-start gap-3">
            <input type="checkbox" checked={notify} onChange={(e) => setNotify(e.target.checked)}
                   className="mt-1 h-4 w-4 shrink-0 cursor-pointer accent-[rgb(var(--violet))]" />
            <span>
              <span className="block text-[.875rem] text-ink">
                Email me when a new version of Astralis is released
              </span>
              <span className="mt-0.5 block font-mono text-[.625rem] leading-relaxed text-ink-3">
                optional · off unless you tick it · a short note per release, nothing else ·
                change it any time under You
              </span>
            </span>
          </label>

          {error && <p role="alert" className="mt-5 font-mono text-[.75rem] text-amber">{error}</p>}

          <button type="submit" className="btn-primary mt-7 w-full sm:w-auto"
                  disabled={busy || !email.trim()}>
            {busy ? "Sending…" : "Send me a code"}
          </button>

          <Privacy />
        </form>
      ) : (
        <div className="animate-rise">
          <p className="label">Check your inbox</p>
          <h1 className="mt-3 font-sans text-[1.875rem] font-semibold leading-tight tracking-[-.025em]">
            Enter the code.
          </h1>
          <p className="mt-3 max-w-measure text-[.9375rem] leading-[1.7] text-ink-2">
            Sent to <span className="font-mono text-ink">{email}</span>. It expires in ten minutes.
            Check spam if it is not there within a minute.
          </p>

          <label htmlFor="code" className="label mt-8 block">6-digit code</label>
          <input
            id="code" ref={codeRef} inputMode="numeric" autoComplete="one-time-code"
            pattern="[0-9]*" maxLength={6} value={code} disabled={busy}
            onChange={(e) => {
              const v = e.target.value.replace(/\D/g, "").slice(0, 6);
              setCode(v);
              if (v.length === 6) void verify(v); // no button press needed
            }}
            className="mt-2.5 w-full max-w-[16rem] rounded-xl border border-line bg-surface px-4 py-3
                       text-center font-mono text-[1.625rem] tracking-[.5em] text-ink outline-none
                       transition-colors focus:border-violet/60 disabled:opacity-50"
            placeholder="······"
          />

          {devCode && (
            <p className="mt-3 font-mono text-[.6875rem] text-cyan">
              dev mode · code {devCode} (this line never appears in production)
            </p>
          )}
          {error && <p role="alert" className="mt-4 font-mono text-[.75rem] text-amber">{error}</p>}
          {busy && <p className="mt-4 font-mono text-[.6875rem] text-ink-3">checking…</p>}

          <div className="mt-8 flex flex-wrap gap-x-6 gap-y-2 font-mono text-[.6875rem]">
            <button onClick={() => void sendCode()} disabled={cooldown > 0 || busy}
                    className="text-violet-2 hover:text-violet disabled:text-ink-3">
              {cooldown > 0 ? `resend in ${cooldown}s` : "send a new code"}
            </button>
            <button onClick={() => { setStep("email"); setError(null); setDevCode(null); }}
                    className="text-ink-3 hover:text-ink-2">
              use a different email
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * The disclaimer, stated as commitments the system actually enforces — not as
 * reassurance. Each line maps to something in the code: /me is the only
 * endpoint that returns an email, the peers query never selects it, and the
 * mail package is the only thing that sends to it.
 */
function Privacy() {
  const lines = [
    ["Nobody else here ever sees it.", "Other people see only the name you choose next. That applies in the circle, beside your messages, and everywhere else. Your email is never shown, listed, or linked to that name for anyone but you."],
    ["It is only for signing in.", "And for release notes, if you ticked the box. No newsletters, no marketing, never shared or sold."],
    ["One provider handles delivery.", "Codes are sent through Resend, an email service, which sees the address only to deliver the message."],
    ["It goes when you go.", "Delete your account under You and the address is erased with everything else."],
  ];
  return (
    <section aria-label="How your email is handled"
             data-butterfly-clear
             className="mt-10 rounded-xl border border-line bg-surface/60 p-5">
      <p className="label">Your email stays yours</p>
      <ul className="mt-4 space-y-3">
        {lines.map(([t, d]) => (
          <li key={t} className="flex gap-3">
            <span aria-hidden className="mt-[.65em] h-px w-3 shrink-0 bg-violet/60" />
            <span className="text-[.8125rem] leading-relaxed text-ink-2">
              <b className="font-medium text-ink">{t}</b> {d}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
