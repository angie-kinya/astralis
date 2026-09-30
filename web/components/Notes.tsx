"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { IconPlus } from "@/components/icons";
import { api, type Note, userMessage } from "@/lib/api";
import { relative } from "@/lib/format";

/**
 * A private notepad.
 *
 * The important property is negative: nothing written here is embedded,
 * retrieved, summarised, or sent to any model. It is the one surface in
 * Astralis where what you write goes nowhere at all — which is exactly why
 * some things get written down only here.
 */
export default function Notes() {
  const [notes, setNotes] = useState<Note[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [status, setStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [error, setError] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>();

  const active = notes.find((n) => n.id === activeId) ?? null;

  useEffect(() => {
    (async () => {
      try {
        const { notes } = await api<{ notes: Note[] }>("/notes");
        setNotes(notes);
        if (notes[0]) select(notes[0]);
      } catch (e) { setError(msg(e)); }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function select(n: Note) {
    setActiveId(n.id); setTitle(n.title); setBody(n.body); setStatus("idle");
  }

  const save = useCallback(async (id: string, t: string, b: string) => {
    setStatus("saving");
    try {
      await api(`/notes/${id}`, { method: "PUT", body: JSON.stringify({ title: t, body: b }) });
      setNotes((prev) => prev.map((n) => n.id === id
        ? { ...n, title: t, body: b, updatedAt: new Date().toISOString() } : n));
      setStatus("saved");
    } catch (e) { setError(msg(e)); setStatus("idle"); }
  }, []);

  // Debounced autosave. Asking someone to remember to press save while they
  // are thinking about something difficult is a design failure.
  useEffect(() => {
    if (!activeId || status === "saving") return;
    if (active && active.title === title && active.body === body) return;
    clearTimeout(timer.current);
    timer.current = setTimeout(() => void save(activeId, title, body), 800);
    return () => clearTimeout(timer.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [title, body, activeId]);

  async function create() {
    try {
      const n = await api<Note>("/notes", { method: "POST" });
      setNotes((prev) => [n, ...prev]);
      select(n);
    } catch (e) { setError(msg(e)); }
  }

  async function remove(id: string) {
    try {
      await api(`/notes/${id}`, { method: "DELETE" });
      const rest = notes.filter((n) => n.id !== id);
      setNotes(rest);
      if (activeId === id) { rest[0] ? select(rest[0]) : (setActiveId(null), setTitle(""), setBody("")); }
    } catch (e) { setError(msg(e)); }
  }

  return (
    <div className="flex h-full">
      <aside aria-label="Notes" className="flex w-52 shrink-0 flex-col border-r border-line bg-surface max-sm:w-36">
        <div className="p-2.5">
          <button onClick={() => void create()} className="btn-quiet w-full justify-start !py-2">
            <IconPlus className="h-3.5 w-3.5" /> New note
          </button>
        </div>
        <ul className="min-h-0 flex-1 overflow-y-auto p-2">
          {notes.length === 0 && (
            <li className="px-1 py-2 font-mono text-[.625rem] leading-relaxed text-ink-3">
              nothing yet
            </li>
          )}
          {notes.map((n) => (
            <li key={n.id}>
              <button onClick={() => select(n)}
                      aria-current={n.id === activeId ? "true" : undefined}
                      className={`w-full rounded-lg px-2.5 py-2 text-left transition-colors
                                  ${n.id === activeId ? "bg-raised" : "hover:bg-raised/60"}`}>
                <span className="block truncate font-sans text-[.8125rem] text-ink">
                  {n.title || "Untitled"}
                </span>
                <span className="mt-0.5 block font-mono text-[.5625rem] text-ink-3">
                  {relative(n.updatedAt)}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col">
        {!active ? (
          <div className="grid h-full place-items-center px-6 text-center">
            <p className="max-w-measure font-sans text-[.875rem] leading-relaxed text-ink-3">
              A private notepad. Nothing here is read by Astralis, embedded, or sent anywhere.
            </p>
          </div>
        ) : (
          <>
            <div className="flex items-center gap-3 border-b border-line px-5 py-2.5">
              <input value={title} onChange={(e) => setTitle(e.target.value.slice(0, 120))}
                     placeholder="Untitled" aria-label="Note title"
                     className="min-w-0 flex-1 bg-transparent font-sans text-[.9375rem] font-medium
                                text-ink outline-none placeholder:text-ink-3/60" />
              <span className="label shrink-0">
                {status === "saving" ? "saving…" : status === "saved" ? "saved" : "private"}
              </span>
              <button onClick={() => void remove(active.id)}
                      className="shrink-0 font-mono text-[.5625rem] uppercase tracking-[.1em]
                                 text-ink-3 underline-offset-2 hover:text-amber hover:underline">
                delete
              </button>
            </div>

            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value.slice(0, 20000))}
              aria-label="Note body"
              placeholder="Write whatever you like. It stays here."
              className="min-h-0 flex-1 resize-none bg-transparent px-5 py-4 font-mono
                         text-[.8125rem] leading-[1.85] text-ink outline-none
                         placeholder:text-ink-3/60"
            />
            {error && <p role="alert" className="px-5 pb-3 font-mono text-[.75rem] text-amber">{error}</p>}
          </>
        )}
      </main>
    </div>
  );
}

const msg = (e: unknown) => userMessage(e);
