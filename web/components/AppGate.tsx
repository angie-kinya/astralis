"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";

import Consent from "@/components/Consent";
import Onboarding from "@/components/Onboarding";
import Rail from "@/components/Rail";
import ThemeToggle from "@/components/ThemeToggle";
import Wordmark from "@/components/Wordmark";
import { ApiError, api, type Me, userMessage } from "@/lib/api";

type Ctx = { me: Me; refresh: () => Promise<void>; setMe: (m: Me) => void };
const MeContext = createContext<Ctx | null>(null);

/** Every module reads identity from here rather than refetching /me. */
export function useMe() {
  const v = useContext(MeContext);
  if (!v) throw new Error("useMe outside AppGate");
  return v;
}

/**
 * Two gates, in order: consent, then a name.
 *
 * Both are enforced independently by the API — requireConsent is middleware,
 * and the handle endpoints are the only ones reachable without a handle. This
 * component is the courteous version of the same rules, not the rules
 * themselves.
 */
export default function AppGate({ children }: { children: React.ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [unread, setUnread] = useState(0);

  const refresh = useCallback(async () => {
    try { setMe(await api<Me>("/me")); }
    catch (e) {
      if (e instanceof ApiError && e.signedOut) return; // api() is already redirecting
      setError(userMessage(e, "Could not reach Astralis. Try refreshing."));
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  // Poll for unread peer messages. Polling rather than websockets is a
  // deliberate call: it costs one cheap query every 20s, survives free-tier
  // hosting that sleeps, and needs no connection state to get wrong.
  useEffect(() => {
    if (!me?.consented || !me.handle) return;
    let alive = true;
    const tick = async () => {
      try {
        const { unread } = await api<{ unread: number }>("/peers/unread");
        if (alive) setUnread(unread);
      } catch { /* transient; the next tick retries */ }
    };
    void tick();
    const id = setInterval(tick, 20_000);
    return () => { alive = false; clearInterval(id); };
  }, [me?.consented, me?.handle]);

  if (error) return <Centered tone="warn">{error}</Centered>;
  if (!me)   return <Centered>Connecting…</Centered>;

  // Order: email (already proven by now) -> a name -> the terms -> Astralis.
  // The name comes first because it is the first thing Astralis says to you;
  // the terms come before anything you say to it is stored.
  if (!me.handle) {
    return <Solo><Onboarding onDone={(handle) => setMe({ ...me, handle })} /></Solo>;
  }
  if (!me.consented) {
    return <Solo><Consent termsUpdatedAt={me.termsUpdatedAt} onAccepted={() => void refresh()} /></Solo>;
  }

  return (
    <MeContext.Provider value={{ me, refresh, setMe }}>
      <div className="flex h-dvh flex-col md:flex-row">
        <Rail unread={unread} />
        <div className="flex min-h-0 min-w-0 flex-1 flex-col max-md:pb-[3.75rem]">
          <header className="flex shrink-0 items-center gap-4 border-b border-line px-4 py-2.5">
            <Wordmark withText={false} />
            <span className="font-mono text-[.6875rem] text-ink-3">
              signed in as <span className="text-ink-2">{me.handle}</span>
            </span>
            <div className="ml-auto flex items-center gap-1">
              <ThemeToggle />
            </div>
          </header>
          <div className="min-h-0 flex-1">{children}</div>
          <footer role="note"
                  className="shrink-0 border-t border-line px-4 py-1.5 text-center
                             font-mono text-[.625rem] leading-relaxed text-ink-3">
            Experimental research project. Not therapy, diagnosis, or a crisis service.
            <span className="hidden sm:inline"> · 999 / 112 · Befrienders Kenya +254 722 178 177</span>
          </footer>
        </div>
      </div>
    </MeContext.Provider>
  );
}

function Solo({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-dvh flex-col">
      <header className="flex shrink-0 items-center justify-between border-b border-line px-5 py-3">
        <Wordmark />
        <ThemeToggle />
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
    </div>
  );
}

function Centered({ children, tone }: { children: React.ReactNode; tone?: "warn" }) {
  return (
    <div className="grid h-dvh place-items-center px-6">
      <p className={`font-mono text-[.8125rem] ${tone === "warn" ? "text-amber" : "text-ink-3"}`}>
        {children}
      </p>
    </div>
  );
}
