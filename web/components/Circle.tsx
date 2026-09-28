"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import Composer from "@/components/Composer";
import CrisisNotice from "@/components/CrisisNotice";
import { useMe } from "@/components/AppGate";
import { IconBack, IconCircle } from "@/components/icons";
import { api, type Peer, type PeerMessage } from "@/lib/api";
import { relative, timeOfDay } from "@/lib/format";

/**
 * The circle: other people using Astralis, under the names they chose.
 *
 * Three decisions worth knowing about:
 *
 *  1. Opt-in, and reversible. Nobody appears here by accident, and leaving
 *     removes you from everyone's list immediately.
 *  2. Presence is derived, not stored. "Online" means "made a request in the
 *     last two minutes", computed when the list is read — so a crashed tab
 *     cannot leave someone showing as available forever.
 *  3. Messages are asynchronous by design. There is no typing indicator and no
 *     read-receipt pressure. Someone offline still receives what you send and
 *     answers when they are back. That is the honest model for peer support
 *     between strangers in different time zones with jobs.
 */
export default function Circle() {
  const { me, setMe } = useMe();
  const [peers, setPeers] = useState<Peer[]>([]);
  const [active, setActive] = useState<Peer | null>(null);
  const [thread, setThread] = useState<PeerMessage[]>([]);
  const [crisis, setCrisis] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const bottom = useRef<HTMLDivElement>(null);

  const loadPeers = useCallback(async () => {
    try { setPeers((await api<{ peers: Peer[] }>("/peers")).peers); }
    catch (e) { setError(msg(e)); }
  }, []);

  const loadThread = useCallback(async (peerId: string) => {
    try {
      const { messages } = await api<{ messages: PeerMessage[] }>(`/peers/${peerId}/messages`);
      setThread(messages);
    } catch (e) { setError(msg(e)); }
  }, []);

  useEffect(() => { if (me.peerOptIn) void loadPeers(); }, [me.peerOptIn, loadPeers]);

  // Poll: the peer list for presence, the open thread for new messages.
  // Cheap queries on a 15s cadence beat a websocket this project does not
  // need and a free tier would sleep through anyway.
  useEffect(() => {
    if (!me.peerOptIn) return;
    const id = setInterval(() => {
      void loadPeers();
      if (active) void loadThread(active.id);
    }, 15_000);
    return () => clearInterval(id);
  }, [me.peerOptIn, active, loadPeers, loadThread]);

  useEffect(() => { bottom.current?.scrollIntoView({ behavior: "smooth" }); }, [thread]);

  async function openThread(p: Peer) {
    setActive(p); setThread([]); setCrisis(null); setError(null);
    await loadThread(p.id);
    setPeers((prev) => prev.map((x) => (x.id === p.id ? { ...x, unread: 0 } : x)));
  }

  async function send(body: string) {
    if (!active) return;
    setBusy(true); setError(null); setCrisis(null);
    const optimistic: PeerMessage = {
      id: `tmp-${Date.now()}`, mine: true, body,
      createdAt: new Date().toISOString(), readAt: null,
    };
    setThread((t) => [...t, optimistic]);
    try {
      const res = await api<{ delivered: boolean; crisis: boolean; reply?: string }>(
        `/peers/${active.id}/messages`, { method: "POST", body: JSON.stringify({ body }) });
      if (res.crisis) {
        // Never delivered. The sender gets the same card the agent would give.
        setThread((t) => t.filter((m) => m.id !== optimistic.id));
        setCrisis(res.reply ?? null);
      } else {
        await loadThread(active.id);
      }
    } catch (e) {
      setThread((t) => t.filter((m) => m.id !== optimistic.id));
      setError(msg(e));
    } finally { setBusy(false); }
  }

  async function setOptIn(value: boolean) {
    try {
      await api("/me/peer-opt-in", { method: "POST", body: JSON.stringify({ peerOptIn: value }) });
      setMe({ ...me, peerOptIn: value });
      if (value) await loadPeers(); else { setPeers([]); setActive(null); }
    } catch (e) { setError(msg(e)); }
  }

  if (!me.peerOptIn) return <OptInGate onJoin={() => void setOptIn(true)} error={error} />;

  return (
    <div className="flex h-full">
      {/* Peer list. Hidden on mobile once a thread is open. */}
      <aside aria-label="People"
             className={`w-full shrink-0 flex-col border-r border-line bg-surface
                         sm:flex sm:w-64 ${active ? "hidden sm:flex" : "flex"}`}>
        <div className="flex items-center justify-between gap-2 border-b border-line px-3 py-2.5">
          <span className="label">
            {peers.filter((p) => p.online).length} online · {peers.length} here
          </span>
          <button onClick={() => void setOptIn(false)}
                  className="font-mono text-[.5625rem] uppercase tracking-[.1em] text-ink-3
                             underline-offset-2 hover:text-amber hover:underline">
            leave
          </button>
        </div>

        <ul className="min-h-0 flex-1 overflow-y-auto p-2">
          {peers.length === 0 && (
            <li className="px-1 py-3 font-mono text-[.6875rem] leading-relaxed text-ink-3">
              nobody else has joined yet. you will show up here for them as{" "}
              <span className="text-ink-2">{me.handle}</span>.
            </li>
          )}
          {peers.map((p) => (
            <li key={p.id}>
              <button onClick={() => void openThread(p)}
                      aria-current={active?.id === p.id ? "true" : undefined}
                      className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left
                                  transition-colors duration-150 ease-swift
                                  ${active?.id === p.id ? "bg-raised" : "hover:bg-raised/60"}`}>
                <span className={p.online ? "dot-on" : "dot-off"} aria-hidden />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-mono text-[.8125rem] text-ink">{p.handle}</span>
                  <span className="block font-mono text-[.5625rem] text-ink-3">
                    {p.online ? "online now" : relative(p.lastSeen)}
                  </span>
                </span>
                {p.unread > 0 && (
                  <span className="grid h-4 min-w-4 place-items-center rounded-full bg-cyan px-1
                                   font-mono text-[.5625rem] font-bold text-bg">{p.unread}</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      </aside>

      {/* Thread */}
      <main className={`min-w-0 flex-1 flex-col ${active ? "flex" : "hidden sm:flex"}`}>
        {!active ? (
          <div className="grid h-full place-items-center px-6 text-center">
            <div>
              <IconCircle className="mx-auto h-6 w-6 text-ink-3" />
              <p className="mt-3 font-mono text-[.75rem] text-ink-3">
                pick someone to write to
              </p>
              <p className="mx-auto mt-2 max-w-measure font-sans text-[.8125rem] leading-relaxed text-ink-3">
                Messages wait for people who are offline. Nobody is expected to reply quickly.
              </p>
            </div>
          </div>
        ) : (
          <>
            <header className="flex items-center gap-2.5 border-b border-line px-4 py-2.5">
              <button onClick={() => setActive(null)} aria-label="Back to people"
                      className="rounded-lg p-1 text-ink-3 hover:bg-raised hover:text-ink sm:hidden">
                <IconBack className="h-4 w-4" />
              </button>
              <span className={active.online ? "dot-on" : "dot-off"} aria-hidden />
              <span className="font-mono text-[.8125rem] text-ink">{active.handle}</span>
              <span className="font-mono text-[.5625rem] text-ink-3">
                {active.online ? "online" : relative(active.lastSeen)}
              </span>
            </header>

            <div className="min-h-0 flex-1 overflow-y-auto">
              <div className="mx-auto max-w-2xl space-y-4 px-5 py-6">
                {thread.length === 0 && !crisis && (
                  <p className="font-mono text-[.6875rem] leading-relaxed text-ink-3">
                    nothing here yet. you are both anonymous — say what is actually going on.
                  </p>
                )}

                {thread.map((m) => (
                  <div key={m.id} className={`animate-rise flex ${m.mine ? "justify-end" : ""}`}>
                    <div className={`max-w-[80%] rounded-xl px-3.5 py-2.5 font-sans text-[.875rem]
                                     leading-relaxed ${m.mine
                                       ? "rounded-br-sm bg-violet/15 text-ink"
                                       : "rounded-bl-sm border border-line bg-surface text-ink"}`}>
                      <p className="whitespace-pre-wrap">{m.body}</p>
                      <p className="mt-1.5 font-mono text-[.5625rem] text-ink-3">
                        {timeOfDay(m.createdAt)}{m.mine && m.readAt ? " · read" : ""}
                      </p>
                    </div>
                  </div>
                ))}

                {crisis && <CrisisNotice text={crisis} />}
                {error && (
                  <p role="alert" className="font-mono text-[.75rem] text-amber">{error}</p>
                )}
                <div ref={bottom} className="h-1" />
              </div>
            </div>

            <div className="border-t border-line px-5 py-3.5">
              <div className="mx-auto max-w-2xl">
                <Composer onSend={(t) => void send(t)} disabled={busy} max={2000}
                          placeholder={`Write to ${active.handle}…`} />
              </div>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function OptInGate({ onJoin, error }: { onJoin: () => void; error: string | null }) {
  return (
    <div className="mx-auto flex h-full max-w-xl flex-col justify-center px-6 py-10">
      <p className="label">The circle</p>
      <h1 className="mt-3 font-sans text-[1.75rem] font-semibold leading-tight tracking-[-.025em]">
        Other people are doing this too.
      </h1>
      <p className="mt-4 max-w-measure text-[.9375rem] leading-[1.72] text-ink-2">
        Astralis can only ever be a research tool. Sometimes what actually helps is
        someone who recognises the week you are having — a nurse, a founder, a
        teacher, someone freelancing and not sleeping.
      </p>

      <ul className="mt-7 space-y-3">
        {[
          ["Only your chosen name is shown", `You appear as your chosen name and nothing else. Your email is never shown, listed, or linked to it.`],
          ["Messages wait", "Write to someone who is offline and they will see it when they return. Nobody owes you a fast reply, and you owe nobody one."],
          ["The same safety filter applies", "Crisis language is stopped before it is delivered. Peer support is not crisis support, and neither of you should be put in that position."],
          ["Leave whenever", "One click removes you from everyone's list."],
        ].map(([t, d]) => (
          <li key={t} className="flex gap-3">
            <span aria-hidden className="mt-[.65em] h-px w-3 shrink-0 bg-violet/60" />
            <span className="text-[.875rem] leading-relaxed text-ink-2">
              <b className="font-medium text-ink">{t}.</b> {d}
            </span>
          </li>
        ))}
      </ul>

      {error && <p className="mt-5 font-mono text-[.75rem] text-amber">{error}</p>}

      <div className="mt-8">
        <button className="btn-primary" onClick={onJoin}>Join the circle</button>
      </div>
    </div>
  );
}

function msg(e: unknown) { return e instanceof Error ? e.message : "Something went wrong."; }
