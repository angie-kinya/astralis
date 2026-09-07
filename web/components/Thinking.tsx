/**
 * Not three bouncing dots. A slowly breathing skeleton at the same measure and
 * leading as the prose that will replace it, so the layout does not jump and
 * the wait feels like something being written rather than something buffering.
 */
export default function Thinking() {
  return (
    <div className="border-l border-line pl-5 sm:pl-6" role="status" aria-live="polite">
      <span className="sr-only">Astralis is composing a reply</span>
      <div className="max-w-reading space-y-3" aria-hidden>
        {["92%", "100%", "68%"].map((w, i) => (
          <div
            key={i}
            className="h-[0.6875rem] animate-shimmer rounded-full bg-line"
            style={{ width: w, animationDelay: `${i * 180}ms` }}
          />
        ))}
      </div>
      <p className="mt-4 font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-ink-3">
        Searching the library
      </p>
    </div>
  );
}
