"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import {
  IconAstralis, IconCircle, IconNotes, IconPractice, IconYou,
} from "@/components/icons";

const NAV = [
  { href: "/chat",      label: "Astralis",  Icon: IconAstralis },
  { href: "/circle",    label: "Circle",    Icon: IconCircle },
  { href: "/resources", label: "Practices", Icon: IconPractice },
  { href: "/notes",     label: "Notes",     Icon: IconNotes },
  { href: "/settings",  label: "You",       Icon: IconYou },
];

/**
 * The module switcher. A narrow icon rail on desktop, a bottom bar on mobile —
 * same five destinations, same order, so the mental map does not change with
 * the viewport.
 *
 * `unread` is surfaced here rather than inside Circle because the whole point
 * of a badge is to be visible from somewhere else.
 */
export default function Rail({ unread }: { unread: number }) {
  const path = usePathname();

  return (
    <nav
      aria-label="Modules"
      className="z-30 flex shrink-0 gap-1 border-line bg-surface
                 max-md:fixed max-md:inset-x-0 max-md:bottom-0 max-md:border-t max-md:px-2 max-md:py-1.5
                 md:w-[4.5rem] md:flex-col md:items-center md:border-r md:py-3"
    >
      {NAV.map(({ href, label, Icon }) => {
        const active = path.startsWith(href);
        const badge = href === "/circle" && unread > 0;
        return (
          <Link
            key={href}
            href={href}
            aria-current={active ? "page" : undefined}
            className={`group relative flex flex-1 flex-col items-center gap-1 rounded-lg
                        px-2 py-2 transition-colors duration-150 ease-swift md:flex-none md:w-14
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
            <span className="font-mono text-[.5625rem] uppercase tracking-[.1em]">{label}</span>
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
