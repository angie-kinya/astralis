"use client";

import type { SessionSummary } from "@/lib/api";
import { dayBucket, timeOfDay } from "@/lib/format";

/**
 * Sessions grouped the way a person remembers them — Today, Yesterday, This
 * week — rather than as a flat list of timestamps. Continuity is the point of
 * the whole episodic-memory design; the sidebar should make it visible.
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
    const key = dayBucket(s.lastActivityAt);
    (acc[key] ??= []).push(s);
    return acc;
  }, {});

  return (
    <>
      {/* Mobile scrim */}
      {open && (
        <button aria-label="Close conversation list" onClick={onClose}
                className="fixed inset-0 z-20 bg-ground/70 backdrop-blur-sm md:hidden" />
      )}

      <aside
        aria-label="Your conversations"
        className={`z-30 flex w-64 shrink-0 flex-col border-r border-line bg-surface
                    md:relative md:translate-x-0
                    ${open ? "fixed inset-y-0 left-0 translate-x-0" : "fixed inset-y-0 -translate-x-full md:flex"}
                    transition-transform duration-200`}
      >
        <div className="p-3">
          <button onClick={onNew} className="btn-quiet w-full justify-start">
            <svg viewBox="0 0 16 16" className="h-3.5 w-3.5" fill="none"
                 stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" aria-hidden>
              <path d="M8 3.5v9M3.5 8h9" />
            </svg>
            New conversation
          </button>
        </div>

        <nav className="min-h-0 flex-1 overflow-y-auto px-3 pb-4">
          {sessions.length === 0 && (
            <p className="px-1 py-2 text-[0.8125rem] text-ink-3">No conversations yet.</p>
          )}

          {Object.entries(groups).map(([bucket, items]) => (
            <div key={bucket} className="mb-5">
              <p className="eyebrow px-1 pb-2">{bucket}</p>
              <ul className="space-y-0.5">
                {items.map((s) => {
                  const active = s.id === activeId;
                  return (
                    <li key={s.id}>
                      <button
                        onClick={() => { onSelect(s.id); onClose(); }}
                        aria-current={active ? "true" : undefined}
                        className={`w-full rounded-lg px-2.5 py-2 text-left transition-colors
                          ${active ? "bg-raised" : "hover:bg-raised/60"}`}
                      >
                        <span className={`block truncate text-[0.8125rem] leading-snug
                          ${active ? "text-ink" : "text-ink-2"}`}>
                          {s.title}
                        </span>
                        <span className="mt-0.5 block font-mono text-[0.625rem] text-ink-3">
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
