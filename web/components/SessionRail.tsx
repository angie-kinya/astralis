"use client";

import { IconPlus } from "@/components/icons";
import type { SessionSummary } from "@/lib/api";
import { dayBucket, timeOfDay } from "@/lib/format";

/**
 * Sessions grouped the way a person remembers them — Today, Yesterday, This
 * week — rather than as a flat list of timestamps. Continuity between sessions
 * is the whole point of the episodic-memory design; the sidebar should make it
 * visible rather than burying it in ISO strings.
 */
export default function SessionRail({
  sessions, activeId, onSelect, onNew, open, onClose,
}: {
  sessions: SessionSummary[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  open: boolean;
  onClose: () => void;
}) {
  const groups = sessions.reduce<Record<string, SessionSummary[]>>((acc, s) => {
    (acc[dayBucket(s.lastActivityAt)] ??= []).push(s);
    return acc;
  }, {});

  return (
    <>
      {open && (
        <button aria-label="Close conversation list" onClick={onClose}
                className="fixed inset-0 z-20 bg-bg/70 backdrop-blur-sm lg:hidden" />
      )}

      <aside aria-label="Your conversations"
             className={`z-30 flex w-60 shrink-0 flex-col border-r border-line bg-surface
                         transition-transform duration-200 ease-swift lg:relative lg:translate-x-0
                         ${open ? "fixed inset-y-0 left-0 translate-x-0"
                                : "fixed inset-y-0 -translate-x-full lg:flex"}`}>
        <div className="p-2.5">
          <button onClick={onNew} className="btn-quiet w-full justify-start !py-2">
            <IconPlus className="h-3.5 w-3.5" />
            New conversation
          </button>
        </div>

        <nav className="min-h-0 flex-1 overflow-y-auto px-2.5 pb-4">
          {sessions.length === 0 && (
            <p className="px-1 py-2 font-mono text-[.6875rem] text-ink-3">no conversations yet</p>
          )}

          {Object.entries(groups).map(([bucket, items]) => (
            <div key={bucket} className="mb-5">
              <p className="label px-1 pb-2">{bucket}</p>
              <ul className="space-y-0.5">
                {items.map((s) => {
                  const active = s.id === activeId;
                  return (
                    <li key={s.id}>
                      <button onClick={() => { onSelect(s.id); onClose(); }}
                              aria-current={active ? "true" : undefined}
                              className={`w-full rounded-lg px-2.5 py-2 text-left transition-colors
                                          duration-150 ease-swift
                                          ${active ? "bg-raised" : "hover:bg-raised/60"}`}>
                        <span className={`block truncate font-sans text-[.8125rem] leading-snug
                                          ${active ? "text-ink" : "text-ink-2"}`}>
                          {s.title}
                        </span>
                        <span className="mt-0.5 block font-mono text-[.5625rem] text-ink-3">
                          {timeOfDay(s.lastActivityAt)}
                        </span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </nav>
      </aside>
    </>
  );
}
