"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import Composer from "@/components/Composer";
import Consent from "@/components/Consent";
import SessionRail from "@/components/SessionRail";
import Thinking from "@/components/Thinking";
import TurnView, { type Turn } from "@/components/Turn";
import {
  ApiError, api, endSessionOnUnload,
  type Me, type SendResult, type SessionSummary, type StoredMessage,
} from "@/lib/api";

const OPENERS = [
  "Tell me how work has actually been, not how you would describe it in a stand-up.",
  "What has been draining you lately — and what have you already tried?",
  "Is there something about work you keep turning over and not resolving?",
];

export default function Workspace() {
  const [me, setMe] = useState<Me | null>(null);
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [railOpen, setRailOpen] = useState(false);

  const bottom = useRef<HTMLDivElement>(null);
  const seq = useRef(0);
  const key = () => `t${seq.current++}`;

  // ------------------------------------------------------------ bootstrap
  //
  // Resume the most recent conversation rather than starting a new one. The
  // obvious implementation — create a session on mount — silently produces an
  // empty session on every page refresh and buries real history under noise.
  useEffect(() => {
    (async () => {
      try {
        const profile = await api<Me>("/me");
        setMe(profile);
        if (!profile.consented) return;

        const { sessions } = await api<{ sessions: SessionSummary[] }>("/sessions");
        setSessions(sessions);
        if (sessions[0]) await open(sessions[0].id);
        else await startNew();
      } catch (e) {
        setError(message(e));
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    bottom.current?.scrollIntoView({ behavior: turns.length > 1 ? "smooth" : "auto" });
  }, [turns, pending]);

  // Ask the API to summarise as the tab goes away. `pagehide` fires in cases
  // `beforeunload` does not — notably iOS Safari and bfcache navigations.
  useEffect(() => {
    if (!activeId) return;
    const end = () => void endSessionOnUnload(activeId);
    window.addEventListener("pagehide", end);
    return () => window.removeEventListener("pagehide", end);
  }, [activeId]);

  // ------------------------------------------------------------ actions
  const open = useCallback(async (id: string) => {
    setActiveId(id);
    setTurns([]);
    setError(null);
    try {
      const { messages } = await api<{ messages: StoredMessage[] }>(`/sessions/${id}/messages`);
      setTurns(messages.map((m) => ({
        key: m.id, role: m.role, content: m.content,
      })));
    } catch (e) {
      setError(message(e));
    }
  }, []);

  async function startNew() {
    try {
      const { id } = await api<{ id: string }>("/sessions", { method: "POST" });
      setActiveId(id);
      setTurns([]);
      setError(null);
      setRailOpen(false);
      setSessions((prev) => [
        { id, title: "New conversation", lastActivityAt: new Date().toISOString() },
        ...prev,
      ]);
    } catch (e) {
      setError(message(e));
    }
  }

  async function send(content: string) {
    if (!activeId) return;
    setError(null);
    setTurns((t) => [...t, { key: key(), role: "user", content }]);
    setPending(true);

    try {
      const res = await api<SendResult>(`/sessions/${activeId}/messages`, {
        method: "POST",
        body: JSON.stringify({ content }),
      });
      setTurns((t) => [...t, {
        key: key(),
        role: "assistant",
        content: res.reply,
        crisis: res.crisis,
        recommendationId: res.recommendationId,
        sources: res.sources ?? [],
      }]);
      // Keep the rail's title/ordering honest without a refetch.
      setSessions((prev) => prev.map((s) => s.id === activeId
        ? { ...s, title: s.title === "New conversation" ? truncate(content, 48) : s.title,
            lastActivityAt: new Date().toISOString() }
        : s));
    } catch (e) {
      if (e instanceof ApiError && e.needsConsent) { setMe((m) => m && { ...m, consented: false }); return; }
      setError(message(e));
    } finally {
      setPending(false);
    }
  }

  // ------------------------------------------------------------ render
  if (error && !me) return <Centered>{error}</Centered>;
  if (!me) return <Centered muted>Loading…</Centered>;

  if (!me.consented) {
    return (
      <Consent
        version={me.consentVersion}
        onAccepted={async () => {
          setMe({ ...me, consented: true });
          const { sessions } = await api<{ sessions: SessionSummary[] }>("/sessions");
          setSessions(sessions);
          sessions[0] ? await open(sessions[0].id) : await startNew();
        }}
      />
    );
  }

  return (
    <div className="flex h-full">
      <SessionRail
        sessions={sessions} activeId={activeId}
        onSelect={(id) => void open(id)} onNew={() => void startNew()}
        open={railOpen} onClose={() => setRailOpen(false)}
      />

      <main className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-3 border-b border-line px-4 py-2 md:hidden">
          <button onClick={() => setRailOpen(true)} className="btn-quiet !px-2.5 !py-1.5"
                  aria-label="Open conversations">
            <svg viewBox="0 0 16 16" className="h-4 w-4" fill="none" stroke="currentColor"
                 strokeWidth="1.5" strokeLinecap="round" aria-hidden>
              <path d="M2.5 4.5h11M2.5 8h11M2.5 11.5h11" />
            </svg>
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-3xl space-y-8 px-5 py-8 sm:px-8">
            {turns.length === 0 && !pending && <Opener />}

            {/* aria-live so a screen-reader user hears the reply arrive. */}
            <div className="space-y-8" aria-live="polite" aria-relevant="additions">
              {turns.map((t) => <TurnView key={t.key} turn={t} />)}
            </div>

            {pending && <Thinking />}
            {error && (
              <p role="alert" className="rounded-lg border border-signal/30 bg-signal/[0.06]
                                          px-4 py-3 text-[0.8125rem] text-signal">
                {error}
              </p>
            )}
            <div ref={bottom} className="h-2" />
          </div>
        </div>

        <div className="border-t border-line px-5 py-4 sm:px-8">
          <div className="mx-auto max-w-3xl">
            <Composer onSend={(t) => void send(t)} disabled={pending || !activeId} />
          </div>
        </div>
      </main>
    </div>
  );
}

// ------------------------------------------------------------------ bits

function Opener() {
  // Stable per mount, so it does not shuffle under the user mid-thought.
  const line = OPENERS[Math.floor(Math.random() * OPENERS.length)]!;
  return (
    <div className="max-w-reading border-l border-line pl-5 sm:pl-6">
      <p className="eyebrow">New conversation</p>
      <p className="prose-astralis mt-3 text-ink-2">{line}</p>
      <p className="mt-5 font-mono text-[0.6875rem] leading-relaxed text-ink-3">
        Whatever you write is grounded against published research before it is answered.
        You can say &ldquo;what does the evidence actually say about this?&rdquo; at any point.
      </p>
    </div>
  );
}

function Centered({ children, muted }: { children: React.ReactNode; muted?: boolean }) {
  return (
    <div className="grid h-full place-items-center px-6">
      <p className={`text-[0.875rem] ${muted ? "text-ink-3" : "text-signal"}`}>{children}</p>
    </div>
  );
}

function message(e: unknown) {
  return e instanceof Error ? e.message : "Something went wrong.";
}

function truncate(s: string, n: number) {
  return [...s].length <= n ? s : [...s].slice(0, n).join("") + "…";
}
