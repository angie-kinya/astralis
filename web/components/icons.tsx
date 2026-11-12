import { MarkShapes } from "@/components/Wordmark";

/** Stroke icons at a single weight, sized by the caller. No icon library. */
const base = {
  viewBox: "0 0 24 24", fill: "none", stroke: "currentColor",
  strokeWidth: 1.5, strokeLinecap: "round" as const, strokeLinejoin: "round" as const,
};

export const IconAstralis = (p: { className?: string }) => (
  <svg viewBox="0 0 24 24" className={p.className} aria-hidden><MarkShapes /></svg>
);

export const IconCircle = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <circle cx="8" cy="9" r="2.6" /><circle cx="16.5" cy="14.5" r="2.6" />
    <path d="M10.2 10.8 14.3 13" />
    <path d="M4 19.2c.7-2 2.2-3 4-3M20 8c-.7-2-2.2-3-4-3" opacity=".55" />
  </svg>
);

export const IconPractice = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <circle cx="12" cy="12" r="8.4" />
    <path d="M14.8 9.2 13.4 13.4 9.2 14.8 10.6 10.6Z" />
  </svg>
);

export const IconNotes = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <path d="M6 3.8h8.5L18.2 7.5V20a.8.8 0 0 1-.8.8H6a.8.8 0 0 1-.8-.8V4.6a.8.8 0 0 1 .8-.8Z" />
    <path d="M14 4v4h4" opacity=".6" />
    <path d="M8.4 12.6h7M8.4 16h4.6" />
  </svg>
);

export const IconYou = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <circle cx="12" cy="9" r="3.2" />
    <path d="M5.5 19.4c1-3 3.5-4.6 6.5-4.6s5.5 1.6 6.5 4.6" />
  </svg>
);

export const IconSend = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <path d="M4.4 12h14M12.6 6.2 18.8 12l-6.2 5.8" />
  </svg>
);

export const IconPlus = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <path d="M12 5.5v13M5.5 12h13" />
  </svg>
);

export const IconMenu = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <path d="M4 7h16M4 12h16M4 17h16" />
  </svg>
);

export const IconBack = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <path d="M14 6.5 8.5 12l5.5 5.5" />
  </svg>
);

export const IconSignOut = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <path d="M14 4.5H7.5a2 2 0 0 0-2 2v11a2 2 0 0 0 2 2H14" />
    <path d="M11 12h9M16.5 8.5 20 12l-3.5 3.5" />
  </svg>
);

export const IconMore = (p: { className?: string }) => (
  <svg {...base} className={p.className} aria-hidden>
    <circle cx="6" cy="12" r=".9" /><circle cx="12" cy="12" r=".9" /><circle cx="18" cy="12" r=".9" />
  </svg>
);
