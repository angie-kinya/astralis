/**
 * The mark is a small orrery: a fixed point with one body in orbit. It nods at
 * "astralis" without a literal star, and it scales to 16px without turning to
 * mush -- which a star with points does not.
 */
export default function Wordmark({ withText = true }: { withText?: boolean }) {
  return (
    <span className="inline-flex items-center gap-2.5">
      <svg viewBox="0 0 24 24" className="h-[18px] w-[18px] text-accent" aria-hidden>
        <ellipse cx="12" cy="12" rx="10" ry="5.5" fill="none"
                 stroke="currentColor" strokeWidth="1.1" opacity="0.45"
                 transform="rotate(-24 12 12)" />
        <circle cx="12" cy="12" r="3" fill="currentColor" />
        <circle cx="21" cy="8.2" r="1.7" fill="currentColor" opacity="0.85" />
      </svg>
      {withText && (
        <span className="font-sans text-[0.9375rem] font-semibold tracking-[-0.015em] text-ink">
          Astralis
        </span>
      )}
    </span>
  );
}
