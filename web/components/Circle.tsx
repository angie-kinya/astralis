"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import { useMe } from "@/components/AppGate";
import Butterfly from "@/components/Butterflies";
import Composer from "@/components/Composer";
import CrisisNotice from "@/components/CrisisNotice";
import { UNREAD_CHANGED } from "@/components/Rail";
import ReportSheet from "@/components/ReportSheet";
import { KindGlyph } from "@/components/game/Glyphs";
import InviteCard from "@/components/game/InviteCard";
import PlayPicker from "@/components/game/PlayPicker";
import { IconBack, IconClose, IconMore, IconPlay } from "@/components/icons";
import {
  ApiError, type GameOptions, type Invite, acceptInvite, api, blockPeer, declineInvite, deletePeerMessage,
  listInvites, type Peer, type PeerMessage, type ReportReason, reportPeer, sendInvite, userMessage,
} from "@/lib/api";
import { useEvent, useEventsConnected } from "@/lib/events";
import { scrollPaneToEnd } from "@/lib/scroll";
import { dateTime, dayKey, dayLabel, relative, timeOfDay } from "@/lib/format";

// a deleted message keeps its row as a tombstone: body is null and deleted is set
type Msg = PeerMessage;
const gone = (m: Msg) => m.deleted || m.body === null;

/**
 * The circle: other people using Astralis, under the names they chose.
 *
 * Decisions worth knowing about:
 *
 *  1. Opt-in, and reversible. Nobody appears here by accident, and leaving
 *     removes you from everyone's list immediately.
 *  2. Presence is derived, not stored. "Online" means "made a request in the
 *     last two minutes", computed when the list is read, so a crashed tab
 *     cannot leave someone showing as available forever.
 *  3. Messages are asynchronous by design. There is no typing indicator.
 *     Someone offline still receives what you send and answers when they are
 *     back. That is the honest model for peer support between strangers in
 *     different time zones with jobs.
 *  4. Pushed, not polled. New, deleted and read messages arrive over the
 *     shared SSE stream (lib/events). Events are hints: anything that matters
 *     is refetched from the server, and a 60 s poll covers a dropped stream.
 *  5. Delete, block and report exist before anything invites more contact.
 *     A block is silent: the blocked person sees "no longer accepting
 *     messages", never that they were blocked.
 *  6. Games (behind me.games) ride the same thread: "play" in the header
 *     opens a small picker, the invite is a card at the end of the thread,
 *     and accepting takes both people to /play/:id. Everything else about
 *     games lives under /play. An invite past its expiry is hidden here at
 *     once; the server only notices on its next cron pass.
 *  7. An open chat always has a way out: the close button (the back arrow
 *     on phones) and esc both return to the list.
 */
export default function Circle() {
  const { me, setMe } = useMe();
  const [peers, setPeers] = useState<Peer[]>([]);
  const [active, setActive] = useState<Peer | null>(null);
  const [thread, setThread] = useState<Msg[]>([]);
  const [crisis, setCrisis] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [menu, setMenu] = useState(false);
  const [picker, setPicker] = useState(false);
  const [confirmBlock, setConfirmBlock] = useState(false);
  const [report, setReport] = useState<{ messageId?: string } | null>(null);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [, setTick] = useState(0);
  const pane = useRef<HTMLDivElement>(null);
  const live = useEventsConnected();
  const router = useRouter();
  const games = me.games;

  // events arrive in closures registered once, so they read the open thread through a ref
  const activeRef = useRef<Peer | null>(null);
  activeRef.current = active;
  const peersRef = useRef<Peer[]>(peers);
  peersRef.current = peers;

  const loadPeers = useCallback(async () => {
    try { setPeers((await api<{ peers: Peer[] }>("/peers")).peers); }
    catch (e) { setError(msg(e)); }
  }, []);

  // reading a thread marks it read on the server, so the rail badge refetches after
  const loadThread = useCallback(async (peerId: string) => {
    try {
      const { messages } = await api<{ messages: Msg[] }>(`/peers/${peerId}/messages`);
      if (activeRef.current?.id === peerId) setThread(messages);
      window.dispatchEvent(new Event(UNREAD_CHANGED));
    } catch (e) { setError(msg(e)); }
  }, []);

  useEffect(() => { if (me.peerOptIn) void loadPeers(); }, [me.peerOptIn, loadPeers]);

  // presence is coarse (two minutes), so the list refreshes slowly whatever the stream does
  useEffect(() => {
    if (!me.peerOptIn) return;
    const id = setInterval(() => void loadPeers(), 60_000);
    return () => clearInterval(id);
  }, [me.peerOptIn, loadPeers]);

  // the open thread only polls while the stream is down
  useEffect(() => {
    if (!me.peerOptIn || live || !active) return;
    const id = setInterval(() => void loadThread(active.id), 60_000);
    return () => clearInterval(id);
  }, [me.peerOptIn, live, active, loadThread]);

  useEvent<{ peerId: string; message: Msg }>("message.new", ({ peerId, message }) => {
    if (activeRef.current?.id === peerId) {
      if (message.mine) {
        // sent from another tab: append unless this tab already has it
        setThread((t) => (t.some((m) => m.id === message.id) ? t : [...t, message]));
      } else {
        void loadThread(peerId); // the fetch marks it read and tells the sender
      }
      return;
    }
    if (message.mine) return;
    if (!peersRef.current.some((p) => p.id === peerId)) { void loadPeers(); return; }
    setPeers((prev) => prev.map((p) => (p.id === peerId ? { ...p, unread: p.unread + 1 } : p)));
  });

  useEvent<{ peerId: string; messageId: string }>("message.deleted", ({ peerId, messageId }) => {
    if (activeRef.current?.id !== peerId) { void loadPeers(); return; }
    setThread((t) => t.map((m) => (m.id === messageId ? { ...m, body: null, deleted: true } : m)));
  });

  useEvent<{ peerId: string }>("message.read", ({ peerId }) => {
    if (activeRef.current?.id !== peerId) return;
    const now = new Date().toISOString();
    setThread((t) => t.map((m) => (m.mine && !m.readAt ? { ...m, readAt: now } : m)));
  });

  // ---------------------------------------------------------- invites
  const loadInvites = useCallback(async () => {
    try { setInvites((await listInvites()).invites); } catch { /* the thread still works without them */ }
  }, []);

  useEffect(() => { if (games && me.peerOptIn) void loadInvites(); }, [games, me.peerOptIn, loadInvites]);

  // same rule as the thread: poll only while the stream is down
  useEffect(() => {
    if (!games || !me.peerOptIn || live) return;
    const id = setInterval(() => void loadInvites(), 60_000);
    return () => clearInterval(id);
  }, [games, me.peerOptIn, live, loadInvites]);

  const upsert = (inv: Invite) =>
    setInvites((all) => [...all.filter((i) => i.id !== inv.id), inv]);

  useEvent<{ invite: Invite }>("invite.new", ({ invite }) => { if (games && invite) upsert(invite); });

  useEvent<{ invite: Invite }>("invite.update", ({ invite }) => {
    if (!games || !invite) return;
    upsert(invite);
    // the sender is waiting on this: take them to the board. the recipient's own tab
    // routes from the accept response, and their other tabs stay where they are
    if (invite.status === "accepted" && invite.gameId && invite.mine) {
      router.push(`/play/${invite.gameId}`);
    }
  });

  const pending = invites.filter((i) => i.status === "pending" && new Date(i.expiresAt).getTime() > Date.now());
  const threadInvites = active ? pending.filter((i) => i.peerId === active.id) : [];

  // re-render while an invite is showing, so one that lapses leaves without a refetch
  useEffect(() => {
    if (pending.length === 0) return;
    const id = setInterval(() => setTick((t) => t + 1), 30_000);
    return () => clearInterval(id);
  }, [pending.length]);

  async function invite(kind: Invite["kind"], options: GameOptions) {
    if (!active) return;
    setError(null); setNotice(null);
    try {
      const { invite: inv } = await sendInvite(active.id, kind, options);
      upsert(inv);
      setPicker(false);
    } catch (e) {
      setPicker(false);
      if (e instanceof ApiError && e.code === "invite_pending") { void loadInvites(); return; }
      if (e instanceof ApiError && e.code === "level_locked") {
        setError(`${active.handle} has not opened that sky yet. A smaller one will work.`);
        return;
      }
      if (e instanceof ApiError && e.code === "peer_unavailable") {
        setError(`${active.handle} is not taking invites right now.`);
        return;
      }
      if (e instanceof ApiError && e.rateLimited) {
        setError("That is a lot of invites for one hour. Try again a little later.");
        return;
      }
      setError(msg(e));
    }
  }

  async function accept(inv: Invite) {
    try {
      const { game } = await acceptInvite(inv.id);
      router.push(`/play/${game.id}`);
    } catch (e) {
      if (e instanceof ApiError && e.code === "invite_gone") {
        setInvites((all) => all.filter((i) => i.id !== inv.id));
        setNotice("That invite has already closed.");
        return;
      }
      setError(msg(e));
    }
  }

  async function decline(inv: Invite) {
    try {
      await declineInvite(inv.id);
      setInvites((all) => all.filter((i) => i.id !== inv.id));
    } catch (e) { setError(msg(e)); }
  }

  useEffect(() => { scrollPaneToEnd(pane.current, true); }, [thread.length, threadInvites.length]);

  function closeThread() {
    setActive(null); activeRef.current = null;
    setThread([]); setCrisis(null); setError(null); setNotice(null);
    setMenu(false); setPicker(false); setConfirmBlock(false);
  }

  // esc closes whatever is on top: a menu or picker first, then the chat itself. a half
  // written message is never thrown away by a stray esc
  useEffect(() => {
    if (!active) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape" || e.defaultPrevented || report) return;
      if (menu || picker) { setMenu(false); setPicker(false); return; }
      const t = e.target as HTMLElement | null;
      if (t && (t.tagName === "TEXTAREA" || t.tagName === "INPUT") && (t as HTMLInputElement).value.trim()) return;
      closeThread();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active, menu, picker, report]);

  async function openThread(p: Peer) {
    setActive(p); activeRef.current = p;
    setThread([]); setCrisis(null); setError(null); setNotice(null);
    setMenu(false); setPicker(false); setConfirmBlock(false);
    await loadThread(p.id);
    setPeers((prev) => prev.map((x) => (x.id === p.id ? { ...x, unread: 0 } : x)));
  }

  async function send(body: string) {
    if (!active) return;
    setBusy(true); setError(null); setCrisis(null);
    const optimistic: Msg = {
      id: `tmp-${Date.now()}`, mine: true, body,
      createdAt: new Date().toISOString(), readAt: null, deleted: false,
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

  // optimistic tombstone, put back if the server says no
  async function remove(m: Msg) {
    if (!active) return;
    setThread((t) => t.map((x) => (x.id === m.id ? { ...x, body: null, deleted: true } : x)));
    try { await deletePeerMessage(active.id, m.id); }
    catch (e) {
      setThread((t) => t.map((x) => (x.id === m.id ? m : x)));
      setError(msg(e));
    }
  }

  async function block() {
    if (!active) return;
    try {
      await blockPeer(active.id);
      const id = active.id;
      setPeers((prev) => prev.filter((p) => p.id !== id));
      closeThread();
    } catch (e) { setError(msg(e)); }
  }

  async function submitReport(reason: ReportReason) {
    if (!active || !report) return;
    try {
      await reportPeer(active.id, reason, report.messageId);
      setReport(null);
      setNotice("Reported. Thank you for saying something.");
    } catch (e) { setReport(null); setError(msg(e)); }
  }

  async function setOptIn(value: boolean) {
    try {
      await api("/me/peer-opt-in", { method: "POST", body: JSON.stringify({ peerOptIn: value }) });
      setMe({ ...me, peerOptIn: value });
      if (value) await loadPeers(); else { setPeers([]); setActive(null); }
    } catch (e) { setError(msg(e)); }
  }

  if (!me.peerOptIn) return <OptInGate onJoin={() => void setOptIn(true)} error={error} games={games} />;

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
                {(() => {
                  const inv = pending.find((i) => i.peerId === p.id && !i.mine);
                  return inv && <KindGlyph kind={inv.kind ?? "constellations"} className="h-3.5 w-3.5 shrink-0 text-violet-2" />;
                })()}
                {p.unread > 0 && (
                  <span className="grid h-4 min-w-4 place-items-center rounded-full bg-cyan px-1
                                   font-mono text-[.5625rem] font-bold text-bg">{p.unread}</span>
                )}
              </button>
            </li>
          ))}
        </ul>

        {/* on a phone the list is the whole screen, so the way to the games lives under it */}
        {games && <PlayLink className="shrink-0 border-t border-line px-3 py-2.5 sm:hidden" />}
      </aside>

      {/* Thread */}
      <main className={`min-w-0 flex-1 flex-col ${active ? "flex" : "hidden sm:flex"}`}>
        {!active ? (
          <div className="grid h-full place-items-center overflow-y-auto px-6 py-8 text-center">
            <div className="w-full">
              {/* Perched, not flying: nobody is waiting on you here. The
                  empty state is a rest, not a prompt to hurry. */}
              <Butterfly pose="rest" className="mx-auto h-6 w-6 text-violet-2/70" />
              <p className="mt-3 font-mono text-[.75rem] text-ink-3">
                pick someone to write to
              </p>
              <p className="mx-auto mt-2 max-w-measure font-sans text-[.8125rem] leading-relaxed text-ink-3">
                Messages wait for people who are offline. Nobody is expected to reply quickly.
              </p>
              {games && <PlayLink className="mx-auto mt-8 w-full max-w-sm" />}
            </div>
          </div>
        ) : (
          <>
            <header className="relative flex items-center gap-2.5 border-b border-line px-4 py-2.5">
              <button onClick={closeThread} aria-label="Back to people"
                      className="-ml-1.5 grid h-9 w-9 place-items-center rounded-lg text-ink-3 hover:bg-raised hover:text-ink sm:hidden">
                <IconBack className="h-4 w-4" />
              </button>
              <span className={active.online ? "dot-on" : "dot-off"} aria-hidden />
              <span className="min-w-0 truncate font-mono text-[.8125rem] text-ink">{active.handle}</span>
              <span className="shrink-0 font-mono text-[.5625rem] text-ink-3 max-[380px]:hidden">
                {active.online ? "online" : relative(active.lastSeen)}
              </span>

              <div className="ml-auto flex shrink-0 items-center gap-1">
                {games && (
                  <button onClick={() => { setPicker((v) => !v); setMenu(false); }}
                          disabled={threadInvites.length > 0}
                          title={threadInvites.length > 0 ? "an invite is already waiting in this chat" : undefined}
                          aria-haspopup="dialog" aria-expanded={picker}
                          className="flex h-9 items-center gap-1.5 rounded-lg border border-violet/35 px-2.5
                                     font-mono text-[.6875rem] text-violet-2 transition-colors duration-150
                                     hover:bg-violet/10 disabled:opacity-45 aria-expanded:bg-violet/10">
                    <IconPlay className="h-4 w-4" />
                    play
                  </button>
                )}
                <button onClick={() => { setMenu((v) => !v); setPicker(false); setConfirmBlock(false); }}
                        aria-label={`Options for ${active.handle}`} aria-haspopup="menu" aria-expanded={menu}
                        className="grid h-9 w-9 place-items-center rounded-lg text-ink-3 hover:bg-raised hover:text-ink">
                  <IconMore className="h-4 w-4" />
                </button>
                <button onClick={closeThread} aria-label={`Close chat with ${active.handle}`} title="close (esc)"
                        className="hidden h-9 w-9 place-items-center rounded-lg text-ink-3 hover:bg-raised hover:text-ink sm:grid">
                  <IconClose className="h-4 w-4" />
                </button>
              </div>
              {picker && (
                <PlayPicker handle={active.handle} onClose={() => setPicker(false)}
                            onInvite={(k, o) => invite(k, o)} />
              )}
              {menu && (
                <div role="menu" aria-label={`Options for ${active.handle}`}
                     onKeyDown={(e) => { if (e.key === "Escape") setMenu(false); }}
                     className="absolute right-3 top-full z-20 mt-1 w-56 rounded-xl border border-line
                                bg-surface p-1.5 shadow-xl">
                  {!confirmBlock ? (
                    <>
                      <MenuItem autoFocus
                                onClick={() => { setMenu(false); setReport({}); }}>
                        report {active.handle}
                      </MenuItem>
                      <MenuItem danger onClick={() => setConfirmBlock(true)}>
                        block {active.handle}
                      </MenuItem>
                    </>
                  ) : (
                    <div className="px-2.5 py-2">
                      <p className="text-[.8125rem] leading-relaxed text-ink-2">
                        You will stop seeing each other here. They will not be told.
                      </p>
                      <div className="mt-3 flex justify-end gap-2">
                        <button className="btn-quiet" onClick={() => setConfirmBlock(false)}>Cancel</button>
                        <button className="btn-danger" autoFocus onClick={() => void block()}>Block</button>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </header>

            <div ref={pane} className="min-h-0 flex-1 overflow-y-auto">
              <div className="mx-auto max-w-2xl space-y-4 px-5 py-6">
                {thread.length === 0 && !crisis && (
                  <div className="max-w-2xl">
                    {/* Nothing said yet, and nothing owed yet either. */}
                    <Butterfly pose="rest" className="mb-3 h-4 w-4 text-violet-2/60" />
                    <p className="font-mono text-[.6875rem] leading-relaxed text-ink-3">
                      nothing here yet. you are both anonymous here, so say what is actually going on.
                    </p>
                  </div>
                )}

                {thread.map((m, i) => (
                  <div key={m.id}>
                    {/* a separator wherever the day changes, the way messaging apps do */}
                    {(i === 0 || dayKey(thread[i - 1]!.createdAt) !== dayKey(m.createdAt)) && (
                      <p role="separator" className="py-2 text-center font-mono text-[.625rem] text-ink-3">
                        {dayLabel(m.createdAt)}
                      </p>
                    )}
                    <Bubble m={m} onDelete={() => void remove(m)}
                            onReport={() => setReport({ messageId: m.id })} />
                  </div>
                ))}

                {threadInvites.map((inv) => (
                  <InviteCard key={inv.id} invite={inv}
                              onAccept={() => accept(inv)} onDecline={() => decline(inv)} />
                ))}

                {crisis && <CrisisNotice text={crisis} />}
                {notice && <p role="status" className="font-mono text-[.75rem] text-ink-3">{notice}</p>}
                {error && (
                  <p role="alert" className="font-mono text-[.75rem] text-amber">{error}</p>
                )}
              </div>
            </div>

            <div className="border-t border-line px-5 py-3.5">
              <div className="mx-auto max-w-2xl">
                <Composer onSend={(t) => void send(t)} disabled={busy} max={2000}
                          placeholder={`Write to ${active.handle}…`} />
              </div>
            </div>

            {report && (
              <ReportSheet handle={active.handle} withMessage={!!report.messageId}
                           onSubmit={submitReport} onClose={() => setReport(null)} />
            )}
          </>
        )}
      </main>
    </div>
  );
}

/**
 * One message. Actions sit behind a small button that shows on hover or
 * keyboard focus, and stays tappable on touch screens; deleting asks once.
 */
function Bubble({ m, onDelete, onReport }: { m: Msg; onDelete: () => void; onReport: () => void }) {
  const [open, setOpen] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const tmp = m.id.startsWith("tmp-");
  const close = () => { setOpen(false); setConfirm(false); };

  return (
    <div className={`group animate-rise flex items-start gap-1 ${m.mine ? "flex-row-reverse" : ""}`}>
      <div className={`max-w-[80%] rounded-xl px-3.5 py-2.5 font-sans text-[.875rem]
                       leading-relaxed ${m.mine
                         ? "rounded-br-sm bg-violet/15 text-ink"
                         : "rounded-bl-sm border border-line bg-surface text-ink"}`}>
        {gone(m) ? (
          <p className="italic text-ink-3">
            {m.mine ? "You deleted this message" : "This message was deleted"}
          </p>
        ) : (
          <p className="whitespace-pre-wrap">{m.body}</p>
        )}
        <time dateTime={m.createdAt} title={dateTime(m.createdAt)}
              className="mt-1.5 block font-mono text-[.5625rem] text-ink-3">
          {timeOfDay(m.createdAt)}{m.mine && m.readAt ? " · read" : ""}
        </time>
      </div>

      {!gone(m) && !tmp && (
        <div className="relative self-center">
          <button onClick={() => (open ? close() : setOpen(true))}
                  aria-label="Message options" aria-haspopup="menu" aria-expanded={open}
                  className="rounded-lg p-1 text-ink-3 opacity-0 transition-opacity duration-150
                             hover:bg-raised hover:text-ink focus-visible:opacity-100
                             group-hover:opacity-100 max-md:opacity-60 aria-expanded:opacity-100">
            <IconMore className="h-3.5 w-3.5" />
          </button>
          {open && (
            <div role="menu" aria-label="Message options"
                 onKeyDown={(e) => { if (e.key === "Escape") close(); }}
                 className={`absolute top-full z-20 mt-1 w-48 rounded-xl border border-line bg-surface
                             p-1.5 shadow-xl ${m.mine ? "right-0" : "left-0"}`}>
              {m.mine ? (
                !confirm ? (
                  <MenuItem autoFocus danger onClick={() => setConfirm(true)}>delete message</MenuItem>
                ) : (
                  <div className="px-2.5 py-2">
                    <p className="text-[.8125rem] leading-relaxed text-ink-2">Delete for both of you?</p>
                    <div className="mt-3 flex justify-end gap-2">
                      <button className="btn-quiet" onClick={close}>Cancel</button>
                      <button className="btn-danger" autoFocus onClick={() => { close(); onDelete(); }}>
                        Delete
                      </button>
                    </div>
                  </div>
                )
              ) : (
                <MenuItem autoFocus onClick={() => { close(); onReport(); }}>report message</MenuItem>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function MenuItem({ children, onClick, danger, autoFocus }: {
  children: React.ReactNode; onClick: () => void; danger?: boolean; autoFocus?: boolean;
}) {
  return (
    <button role="menuitem" onClick={onClick} autoFocus={autoFocus}
            className={`block w-full rounded-lg px-2.5 py-2 text-left font-mono text-[.6875rem]
                        transition-colors duration-150 hover:bg-raised
                        ${danger ? "text-amber" : "text-ink-2 hover:text-ink"}`}>
      {children}
    </button>
  );
}

function OptInGate({ onJoin, error, games }: { onJoin: () => void; error: string | null; games: boolean }) {
  return (
    <div className="h-full overflow-y-auto">
    <div className="mx-auto flex min-h-full max-w-xl flex-col justify-center px-6 py-10">
      <p className="label">The circle</p>
      <h1 className="mt-3 font-sans text-[1.75rem] font-semibold leading-tight tracking-[-.025em]">
        Other people are doing this too.
      </h1>
      <p className="mt-4 max-w-measure text-[.9375rem] leading-[1.72] text-ink-2">
        Astralis can only ever be a research tool. Sometimes what actually helps is
        someone who recognises the week you are having. A nurse, a founder, a
        teacher, someone freelancing and not sleeping.
      </p>

      <ul className="mt-7 space-y-3">
        {[
          ["Only your chosen name is shown", `You appear as your chosen name and nothing else. Your email is never shown, listed, or linked to it.`],
          ["Messages wait", "Write to someone who is offline and they will see it when they return. Nobody owes you a fast reply, and you owe nobody one."],
          ["The same safety filter applies", "Crisis language is stopped before it is delivered. Peer support is not crisis support, and neither of you should be put in that position."],
          ["You stay in control", "Delete anything you sent. Block or report anyone, and they are never told."],
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

      {/* astralis, solo and the puzzles need nobody else, so they do not wait on joining */}
      {games && <PlayLink className="mt-10" />}
    </div>
    </div>
  );
}

const msg = (e: unknown) => userMessage(e);

/** games moved out of the circle; this is the signpost to them */
function PlayLink({ className = "" }: { className?: string }) {
  return (
    <div className={className}>
      <Link href="/play"
            className="flex items-center gap-2.5 rounded-xl border border-line bg-surface/70 px-3 py-2.5 text-left
                       transition-colors duration-150 hover:border-line-2 hover:bg-raised/60">
        <IconPlay className="h-4 w-4 shrink-0 text-violet-2" />
        <span className="min-w-0 flex-1">
          <span className="block font-mono text-[.75rem] text-ink">games live in play</span>
          <span className="block font-mono text-[.5625rem] text-ink-3">your games, the week&apos;s board, and new ones</span>
        </span>
      </Link>
    </div>
  );
}
