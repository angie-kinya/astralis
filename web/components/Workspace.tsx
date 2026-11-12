"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { AstralisMark, Gutter } from "@/components/Speaker";
import Composer from "@/components/Composer";
import EndCard from "@/components/EndCard";
import LastTime from "@/components/LastTime";
import SessionRail from "@/components/SessionRail";
import Thinking from "@/components/Thinking";
import TurnView, { type Turn } from "@/components/Turn";
import { IconMenu } from "@/components/icons";
import {
  ApiError, api, endSessionOnUnload,
  type SendResult, type SessionSummary, type StoredMessage, userMessage,
} from "@/lib/api";
import { useEvent } from "@/lib/events";

const OPENERS = [
  "Tell me how work has actually been. Leave out the version you would give in a stand-up.",
  "What has been draining you lately, and what have you already tried?",
  "Is there something about work you keep turning over and never resolving?",
];

// the end card asks once a conversation has had some substance, and only
// after the person has gone quiet for a while or moved on
const END_CARD_REPLIES = 3;
const END_CARD_IDLE_MS = 4 * 60_000;
const ASKED_KEY = "astralis.rated";

export default function Workspace() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  // null = a fresh conversation that does not exist on the server yet
  const [activeId, setActiveId] = useState<string | null>(null);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [pending, setPending] = useState(false);
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [railOpen, setRailOpen] = useState(false);
  const [endCard, setEndCard] = useState<{ id: string; earlier: boolean } | null>(null);

  const bottom = useRef<HTMLDivElement>(null);
  const seq = useRef(0);
  const key = () => `t${seq.current++}`;
  // refs, so the sse handler and timers read the live values without re-subscribing
  const activeRef = useRef<string | null>(null);
  const pendingRef = useRef(false);
  const turnsRef = useRef<Turn[]>([]);
  activeRef.current = activeId;
  pendingRef.current = pending;
  turnsRef.current = turns;

  // truthful progress from the agent ("reading 2 studies"); anything for
  // another session, or arriving after the reply, is ignored
  useEvent<{ sessionId: string; status: string }>("turn.status", (d) => {
    if (pendingRef.current && d.sessionId === activeRef.current) setStatus(d.status);
  });

  // the end card is due when a conversation has enough replies and was never asked about
  const eligible = useCallback((id: string | null, t: Turn[]) => {
    if (!id || asked().includes(id)) return false;
    return t.filter((x) => x.role === "assistant" && !x.crisis).length >= END_CARD_REPLIES;
  }, []);

  const offerEndCard = useCallback((id: string, earlier: boolean) => {
    markAsked(id);
    setEndCard({ id, earlier });
  }, []);

  // leaving a conversation that deserved the question asks it in the next view
  const leave = useCallback(() => {
    const id = activeRef.current;
    if (eligible(id, turnsRef.current)) offerEndCard(id!, true);
  }, [eligible, offerEndCard]);

  const open = useCallback(async (id: string) => {
    if (id === activeRef.current) return;
    leave();
    setActiveId(id); setTurns([]); setError(null); setStatus(null);
    try {
      const { messages } = await api<{ messages: StoredMessage[] }>(`/sessions/${id}/messages`);
      // ?? []: an empty conversation must render as empty, whatever the server sends.
      setTurns((messages ?? []).map((m) => ({ key: m.id, role: m.role, content: m.content })));
    } catch (e) { setError(msg(e)); }
  }, [leave]);

  // a new conversation is local until its first message, so "new" never
  // leaves an empty session behind on the server
  const startNew = useCallback(() => {
    if (activeRef.current === null) { setRailOpen(false); return; }
    leave();
    setActiveId(null); setTurns([]); setError(null); setStatus(null); setRailOpen(false);
  }, [leave]);

  // returning to astralis always lands on a fresh conversation. continuity
  // is the person's choice: every earlier one is a click away in the rail,
  // and the "last time" line carries the thread without reopening it
  useEffect(() => {
    api<{ sessions: SessionSummary[] }>("/sessions")
      .then(({ sessions }) => setSessions(sessions ?? []))
      .catch((e) => setError(msg(e)));
  }, []);

  useEffect(() => {
    bottom.current?.scrollIntoView({ behavior: turns.length > 1 ? "smooth" : "auto" });
  }, [turns, pending, endCard]);

  // Ask the API to summarise as the tab goes away. `pagehide` fires where
  // `beforeunload` does not — notably iOS Safari and bfcache navigations.
  useEffect(() => {
    if (!activeId) return;
    const end = () => void endSessionOnUnload(activeId);
    window.addEventListener("pagehide", end);
    return () => window.removeEventListener("pagehide", end);
  }, [activeId]);

  // gone quiet after a real conversation: ask once, in place
  useEffect(() => {
    if (pending || endCard || !eligible(activeId, turns)) return;
    const t = setTimeout(() => offerEndCard(activeId!, false), END_CARD_IDLE_MS);
    return () => clearTimeout(t);
  }, [turns, pending, activeId, endCard, eligible, offerEndCard]);

  async function send(content: string) {
    setError(null);
    setTurns((t) => [...t, { key: key(), role: "user", content }]);
    setPending(true); setStatus(null);
    if (endCard?.earlier) setEndCard(null);
    try {
      let id = activeId;
      if (!id) {
        ({ id } = await api<{ id: string }>("/sessions", { method: "POST" }));
        activeRef.current = id; // before the send, so its status events are not dropped
        setActiveId(id);
        setSessions((prev) => [
          { id: id!, title: truncate(content, 44), lastActivityAt: new Date().toISOString() }, ...prev,
        ]);
      }
      const res = await api<SendResult>(`/sessions/${id}/messages`, {
        method: "POST", body: JSON.stringify({ content }),
      });
      setTurns((t) => [...t, {
        key: key(), role: "assistant", content: res.reply, crisis: res.crisis,
        recommendationId: res.recommendationId, sources: res.sources ?? [],
      }]);
      setSessions((prev) => prev.map((s) => s.id === id
        ? { ...s,
            title: s.title === "New conversation" ? truncate(content, 44) : s.title,
            lastActivityAt: new Date().toISOString() }
        : s));
    } catch (e) {
      setError(e instanceof ApiError && e.rateLimited
        ? "You are sending messages faster than Astralis can think. Try again in a minute."
        : msg(e));
    } finally { setPending(false); setStatus(null); }
  }

  const fresh = activeId === null && turns.length === 0;

  return (
    <div className="flex h-full">
      <SessionRail sessions={sessions} activeId={activeId}
                   onSelect={(id) => void open(id)} onNew={startNew}
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
            {endCard?.earlier && (
              <EndCard sessionId={endCard.id} earlier onDone={() => setEndCard(null)} />
            )}
            {fresh && <LastTime />}
            {turns.length === 0 && !pending && <Opener />}

            <div className="space-y-7" aria-live="polite" aria-relevant="additions">
              {turns.map((t) => <TurnView key={t.key} turn={t} />)}
            </div>

            {pending && <Thinking status={status} />}
            {endCard && !endCard.earlier && endCard.id === activeId && (
              <EndCard sessionId={endCard.id} onDone={() => setEndCard(null)} />
            )}
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
            <Composer onSend={(t) => void send(t)} disabled={pending} />
          </div>
        </div>
      </main>
    </div>
  );
}

function Opener() {
  // picked once per mount, so a re-render never swaps the line under the reader
  const [line] = useState(() => OPENERS[Math.floor(Math.random() * OPENERS.length)]!);
  return (
    <div className="flex gap-2.5 sm:gap-3">
      <Gutter><AstralisMark /></Gutter>
      <div className="bubble bubble--astralis max-w-reading">
        <p className="reply text-ink-2">{line}</p>
        <p className="mt-3 font-mono text-[.625rem] leading-relaxed text-ink-3">
          when published research would help, astralis looks it up before it answers.
          ask &ldquo;what does the evidence actually say?&rdquo; at any point.
        </p>
      </div>
    </div>
  );
}

// which sessions were already asked about; per browser, and safe when storage is blocked
function asked(): string[] {
  try { return JSON.parse(localStorage.getItem(ASKED_KEY) ?? "[]") as string[]; } catch { return []; }
}
function markAsked(id: string) {
  try { localStorage.setItem(ASKED_KEY, JSON.stringify([...asked(), id].slice(-50))); } catch { /* fine */ }
}

const msg = (e: unknown) => userMessage(e);
function truncate(s: string, n: number) {
  return [...s].length <= n ? s : [...s].slice(0, n).join("") + "…";
}
