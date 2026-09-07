"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import ThemeToggle from "@/components/ThemeToggle";
import Wordmark from "@/components/Wordmark";

const NAV = [
  { href: "/chat", label: "Conversation" },
  { href: "/settings", label: "Your data" },
];

export default function AppFrame({
  name, children,
}: { name: string; children: React.ReactNode }) {
  const path = usePathname();

  return (
    <div className="flex h-dvh flex-col">
      <header className="flex shrink-0 items-center gap-6 border-b border-line px-4 py-3 sm:px-6">
        <Link href="/chat" className="rounded"><Wordmark /></Link>

        <nav className="flex items-center gap-1" aria-label="Main">
          {NAV.map((item) => {
            const active = path.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={`rounded-lg px-2.5 py-1.5 text-[0.8125rem] transition-colors
                  ${active ? "bg-raised text-ink" : "text-ink-3 hover:text-ink-2"}`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-1.5">
          <span className="hidden font-mono text-[0.6875rem] text-ink-3 sm:inline">{name}</span>
          <ThemeToggle />
          <a
            href="/api/auth/signout"
            className="rounded-lg px-2.5 py-1.5 text-[0.8125rem] text-ink-3 transition-colors hover:text-ink-2"
          >
            Sign out
          </a>
        </div>
      </header>

      <div className="min-h-0 flex-1">{children}</div>

      {/* Persistent, low-contrast, never dismissible. A person mid-conversation
          should be able to glance down and be reminded what this is. */}
      <footer
        role="note"
        className="shrink-0 border-t border-line bg-ground/80 px-4 py-2 text-center
                   font-mono text-[0.6875rem] leading-relaxed text-ink-3 backdrop-blur sm:px-6"
      >
        Experimental research project — not therapy, diagnosis, or a crisis service.
        <span className="hidden sm:inline"> Emergency 999 / 112 · Befrienders Kenya +254 722 178 177</span>
      </footer>
    </div>
  );
}
