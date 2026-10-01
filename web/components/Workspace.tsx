"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import Composer from "@/components/Composer";
import SessionRail from "@/components/SessionRail";
import Thinking from "@/components/Thinking";
import TurnView, { type Turn } from "@/components/Turn";
import { useMe } from "@/components/AppGate";
import { IconMenu } from "@/components/icons";
import {
  ApiError, api, endSessionOnUnload,
  type SendResult, type SessionSummary, type StoredMessage, userMessage,
} from "@/lib/api";

const OPENERS = [
  "Tell me how work has actually been. Leave out the version you would give in a stand-up.",
  "What has been draining you lately, and what have you already tried?",
  "Is there something about work you keep turning over and never resolving?",
];

export default function Workspace() {
  const { me } = useMe();
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [railOpen, setRailOpen] = useState(false);

  const bottom = useRef<HTMLDivElement>(null);
  const seq = useRef(0);
  const key = () => `t${seq.current++}`;

  const open = useCallback(async (id: string) => {
    setActiveId(id); setTurns([]); setError(null);
    try {
      const { messages } = await api<{ messages: StoredMessage[] }>(`/sessions/${id}/messages`);
      // ?? []: an empty conversation must render as empty, whatever the server sends.
      setTurns((messages ?? []).map((m) => ({ key: m.id, role: m.role, content: m.content })));
    } catch (e) { setError(msg(e)); }
  }, []);

  const startNew = useCallback(async () => {
    try {
      const { id } = await api<{ id: string }>("/sessions", { method: "POST" });
      setActiveId(id); setTurns([]); setError(null); setRailOpen(false);
      setSessions((prev) => [
        { id, title: "New conversation", lastActivityAt: new Date().toISOString() }, ...prev,
      ]);
    } catch (e) { setError(msg(e)); }
  }, []);

  // Resume the most recent conversation rather than starting a new one. The
  // obvious implementation — create a session on mount — silently produces an
  // empty session on every page refresh and buries real history under noise.
  useEffect(() => {
    (async () => {
      try {
        const { sessions } = await api<{ sessions: SessionSummary[] }>("/sessions");
        setSessions(sessions);
        if (sessions[0]) await open(sessions[0].id);
        else await startNew();
      } catch (e) { setError(msg(e)); }
    })();
  }, [open, startNew]);

  useEffect(() => {
    bottom.current?.scrollIntoView({ behavior: turns.length > 1 ? "smooth" : "auto" });
  }, [turns, pending]);

  // Ask the API to summarise as the tab goes away. `pagehide` fires where
  // `beforeunload` does not — notably iOS Safari and bfcache navigations.
  useEffect(() => {
    if (!activeId) return;
    const end = () => void endSessionOnUnload(activeId);
    window.addEventListener("pagehide", end);
    return () => window.removeEventListener("pagehide", end);
  }, [activeId]);

  async function send(content: string) {
    if (!activeId) return;
    setError(null);
    setTurns((t) => [...t, { key: key(), role: "user", content }]);
    setPending(true);
    try {
      const res = await api<SendResult>(`/sessions/${activeId}/messages`, {
        method: "POST", body: JSON.stringify({ content }),
      });
      setTurns((t) => [...t, {
        key: key(), role: "assistant", content: res.reply, crisis: res.crisis,
        recommendationId: res.recommendationId, sources: res.sources ?? [],
      }]);
      setSessions((prev) => prev.map((s) => s.id === activeId
        ? { ...s,
            title: s.title === "New conversation" ? truncate(content, 44) : s.title,
            lastActivityAt: new Date().toISOString() }
        : s));
    } catch (e) {
      setError(e instanceof ApiError && e.rateLimited
        ? "You are sending messages faster than Astralis can think. Try again in a minute."
        : msg(e));
    } finally { setPending(false); }
  }

  return (
    <div className="flex h-full">
      <SessionRail sessions={sessions} activeId={activeId}
                   onSelect={(id) => void open(id)} onNew={() => void startNew()}
                   open={railOpen} onClose={() => setRailOpen(false)} />

      <main className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-3 border-b border-line px-4 py-1.5 lg:hidden">
          <button onClick={() => setRailOpen(true)} aria-label="Open conversations"
                  className="rounded-lg p-1.5 text-ink-3 hover:bg-raised hover:text-ink">
            <IconMenu className="h-4 w-4" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          <div data-butterfly-clear="chat" className="mx-auto max-w-3xl space-y-7 px-5 py-7 sm:px-8">
            {turns.length === 0 && !pending && <Opener />}

            <div className="space-y-7" aria-live="polite" aria-relevant="additions">
              {turns.map((t) => <TurnView key={t.key} turn={t} handle={me.handle} />)}
            </div>

            {pending && <Thinking />}
            {error && (
              <p role="alert"
                 className="care rounded-lg border border-amber/30 bg-amber/[.06] px-4 py-3
                            font-mono text-[.75rem] text-amber">{error}</p>
            )}
            <div ref={bottom} className="h-1" />
          </div>
        </div>

        <div className="border-t border-line px-5 py-3.5 sm:px-8">
          <div data-butterfly-clear="chat" className="mx-auto max-w-3xl">
            <Composer onSend={(t) => void send(t)} disabled={pending || !activeId} />
          </div>
        </div>
      </main>
    </div>
  );
}

function Opener() {
  const line = OPENERS[Math.floor(Math.random() * OPENERS.length)]!;
  return (
    <div className="max-w-reading border-l border-violet/45 pl-4 sm:pl-5">
      <p className="mb-2 font-mono text-[.625rem] uppercase tracking-[.14em] text-violet-2">
        astralis <span className="text-ink-3/50">›</span>
      </p>
      <p className="reply text-ink-2">{line}</p>
      <p className="mt-4 font-mono text-[.625rem] leading-relaxed text-ink-3">
        when published research would help, astralis looks it up before it answers.
        ask &ldquo;what does the evidence actually say?&rdquo; at any point.
      </p>
    </div>
  );
}

const msg = (e: unknown) => userMessage(e);
function truncate(s: string, n: number) {
  return [...s].length <= n ? s : [...s].slice(0, n).join("") + "…";
}
