"use client";

import { useEffect, useState } from "react";

type Theme = "dark" | "light";

export default function ThemeToggle({ className = "" }: { className?: string }) {
  const [theme, setTheme] = useState<Theme>("dark");

  // The inline script in the root layout already set the attribute before
  // paint; read it back rather than deciding again and causing a flash.
  useEffect(() => {
    setTheme((document.documentElement.dataset.theme as Theme) ?? "dark");
  }, []);

  function toggle() {
    const next: Theme = theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    document.documentElement.style.colorScheme = next;
    try { localStorage.setItem("astralis-theme", next); } catch { /* private mode */ }
    setTheme(next);
  }

  return (
    <button
      onClick={toggle}
      aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} theme`}
      className={`grid h-8 w-8 place-items-center rounded-lg text-ink-3
                  transition-colors hover:bg-raised hover:text-ink ${className}`}
    >
      <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none"
           stroke="currentColor" strokeWidth="1.6" strokeLinecap="round">
        {theme === "dark" ? (
          <path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a7 7 0 0 0 10.5 10.5Z" />
        ) : (
          <>
            <circle cx="12" cy="12" r="4" />
            <path d="M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M18.4 5.6 17 7M7 17l-1.4 1.4" />
          </>
        )}
      </svg>
    </button>
  );
}
