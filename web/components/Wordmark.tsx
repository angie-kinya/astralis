/**
 * A small orrery: one fixed body, one in orbit. It reads as "astralis" without
 * a literal star, and it survives 16px — which a star with points does not.
 */
export default function Wordmark({ withText = true, className = "" }: {
  withText?: boolean; className?: string;
}) {
  return (
    <span className={`inline-flex items-center gap-2.5 ${className}`}>
      <svg viewBox="0 0 24 24" className="h-[18px] w-[18px] text-violet" aria-hidden>
        <ellipse cx="12" cy="12" rx="10" ry="5.5" fill="none" stroke="currentColor"
                 strokeWidth="1.1" opacity=".45" transform="rotate(-24 12 12)" />
        <circle cx="12" cy="12" r="3" fill="currentColor" />
        <circle cx="21" cy="8.2" r="1.7" fill="currentColor" opacity=".85" />
      </svg>
      {withText && (
        <span className="font-mono text-[.875rem] font-medium tracking-[-.01em] text-ink">
          astralis
        </span>
      )}
    </span>
  );
}
