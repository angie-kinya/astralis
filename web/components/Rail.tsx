"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { useMe } from "@/components/AppGate";
import {
  IconAstralis, IconCircle, IconNotes, IconPlay, IconPractice, IconYou,
} from "@/components/icons";
import { api } from "@/lib/api";
import { useEvent, useEventsConnected } from "@/lib/events";

/** fired by Circle after it opens a thread, since reading marks messages read */
export const UNREAD_CHANGED = "astralis:unread-changed";

/**
 * The unread count, pushed rather than polled. Any peer event (or Circle
 * opening a thread) refetches the one cheap count, so the badge is always the
 * server's number and never drifts from local arithmetic. A 60 s poll covers
 * the moments the stream is down.
 */
function useUnread(): number {
  const [unread, setUnread] = useState(0);
  const live = useEventsConnected();

  const refetch = useCallback(async () => {
    try { setUnread((await api<{ unread: number }>("/peers/unread")).unread); }
    catch { /* transient; the next event or tick retries */ }
  }, []);

  useEvent("message.new", () => void refetch());
  useEvent("message.deleted", () => void refetch());

  useEffect(() => {
    void refetch();
    const on = () => void refetch();
    window.addEventListener(UNREAD_CHANGED, on);
    return () => window.removeEventListener(UNREAD_CHANGED, on);
  }, [refetch]);

  useEffect(() => {
    if (live) return;
    const id = setInterval(() => void refetch(), 60_000);
    return () => clearInterval(id);
  }, [live, refetch]);

  return unread;
}

const NAV = [
  { href: "/chat",      label: "Astralis",  Icon: IconAstralis },
  { href: "/circle",    label: "Circle",    Icon: IconCircle },
  { href: "/play",      label: "Play",      Icon: IconPlay },
  { href: "/resources", label: "Practices", Icon: IconPractice },
  { href: "/notes",     label: "Notes",     Icon: IconNotes },
  { href: "/settings",  label: "You",       Icon: IconYou },
];

/**
 * The module switcher. A narrow icon rail on desktop, a bottom bar on mobile —
 * same destinations (play only when games are on), same order, so the mental map does not change with
 * the viewport.
 *
 * `unread` is surfaced here rather than inside Circle because the whole point
 * of a badge is to be visible from somewhere else.
 */
export default function Rail() {
  const path = usePathname();
  const unread = useUnread();
  const { me } = useMe();
  const nav = me.games ? NAV : NAV.filter((n) => n.href !== "/play");

  return (
    <nav
      aria-label="Modules"
      className="z-30 flex shrink-0 gap-1 border-line bg-surface
                 max-md:fixed max-md:inset-x-0 max-md:bottom-0 max-md:border-t max-md:px-2 max-md:py-1.5
                 md:w-[4.5rem] md:flex-col md:items-center md:border-r md:py-3"
    >
      {nav.map(({ href, label, Icon }) => {
        const active = path.startsWith(href);
        const badge = href === "/circle" && unread > 0;
        return (
          <Link
            key={href}
            href={href}
            aria-current={active ? "page" : undefined}
            className={`group relative flex min-w-0 flex-1 flex-col items-center gap-1 rounded-lg
                        px-0.5 py-2 transition-colors duration-150 ease-swift md:flex-none md:w-14 md:px-2
                        ${active ? "bg-raised text-violet-2" : "text-ink-3 hover:bg-raised/60 hover:text-ink-2"}`}
          >
            <span className="relative">
              <Icon className="h-[1.15rem] w-[1.15rem]" />
              {badge && (
                <span className="absolute -right-1.5 -top-1 grid h-3.5 min-w-3.5 place-items-center
                                 rounded-full bg-cyan px-1 font-mono text-[.5rem] font-bold text-bg">
                  {unread > 9 ? "9+" : unread}
                </span>
              )}
            </span>
            <span className="max-w-full truncate font-mono text-[.5625rem] uppercase tracking-[.1em] max-md:tracking-[.04em]">{label}</span>
            {active && (
              <span aria-hidden
                    className="absolute bg-violet max-md:inset-x-3 max-md:top-0 max-md:h-px
                               md:left-0 md:h-7 md:w-[2px] md:rounded-r" />
            )}
          </Link>
        );
      })}
    </nav>
  );
}
