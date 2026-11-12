"use client";

import { useEffect, useRef, useState } from "react";

/**
 * The server-push half of real time. One EventSource per tab, shared by every
 * component that listens, opened on the first subscriber and closed when the
 * last one leaves, so a page with no listeners holds no connection.
 *
 * Sending never goes through here: the client still POSTs as before. SSE only
 * tells the tab that something changed (message.new, message.deleted,
 * message.read, turn.status). The session cookie is same-origin, so the stream
 * authenticates with no extra work, and EventSource reconnects on its own.
 *
 * Events are hints, not the record. A tab that misses one recovers on its next
 * fetch, which is why callers keep a slow poll while useEventsConnected() is false.
 */

type Handler = (data: unknown) => void;

let source: EventSource | null = null;
let connected = false;
const handlers = new Map<string, Set<Handler>>();
const statusListeners = new Set<(on: boolean) => void>();
// one dispatcher per event type, attached to whichever EventSource is current
const attached = new Set<string>();

function setConnected(on: boolean) {
  if (connected === on) return;
  connected = on;
  statusListeners.forEach((l) => l(on));
}

function dispatch(type: string) {
  return (e: MessageEvent) => {
    let data: unknown = e.data;
    try { data = JSON.parse(e.data); } catch { /* plain text payload, pass as is */ }
    handlers.get(type)?.forEach((h) => h(data));
  };
}

function open() {
  if (source || typeof window === "undefined" || typeof EventSource === "undefined") return;
  source = new EventSource("/api/v1/events", { withCredentials: true });
  attached.clear();
  source.onopen = () => setConnected(true);
  // error fires on every drop; readyState tells a retry (CONNECTING) from a dead stream
  source.onerror = () => setConnected(false);
  handlers.forEach((_, type) => attach(type));
}

function attach(type: string) {
  if (!source || attached.has(type)) return;
  source.addEventListener(type, dispatch(type) as EventListener);
  attached.add(type);
}

function closeIfIdle() {
  if (!source || handlers.size > 0 || statusListeners.size > 0) return;
  source.close();
  source = null;
  attached.clear();
  setConnected(false);
}

/** Subscribe to one event type for the life of the component. */
export function useEvent<T = unknown>(type: string, handler: (data: T) => void): void {
  // the latest handler is read through a ref, so callers can pass an inline
  // closure without resubscribing on every render
  const ref = useRef(handler);
  ref.current = handler;

  useEffect(() => {
    const h: Handler = (d) => ref.current(d as T);
    if (!handlers.has(type)) handlers.set(type, new Set());
    handlers.get(type)!.add(h);
    open();
    attach(type);
    return () => {
      const set = handlers.get(type);
      set?.delete(h);
      if (set && set.size === 0) handlers.delete(type);
      closeIfIdle();
    };
  }, [type]);
}

/** false while the stream is down, so callers fall back to a 60 s poll. */
export function useEventsConnected(): boolean {
  const [on, setOn] = useState(connected);
  useEffect(() => {
    statusListeners.add(setOn);
    open();
    setOn(connected);
    return () => { statusListeners.delete(setOn); closeIfIdle(); };
  }, []);
  return on;
}
