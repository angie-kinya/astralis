import Butterfly from "@/components/Butterflies";

/**
 * Not three bouncing dots. A skeleton at the same measure and leading as the
 * prose that will replace it, so the layout does not jump — and a mono status
 * line that names what is actually happening, because "searching the library"
 * is true and more reassuring than an animation that means nothing.
 */
export default function Thinking() {
  return (
    <div className="animate-fade border-l border-violet/45 pl-4 sm:pl-5" role="status" aria-live="polite">
      {/* Someone is here, and in no hurry: the wings flex slowly while the
          reply is composed — a heartbeat instead of a spinner. */}
      <p className="mb-3 flex items-center gap-2 font-mono text-[.625rem]
                    uppercase tracking-[.14em] text-violet-2">
        <Butterfly pose="hover" className="h-3.5 w-3.5 shrink-0" />
        astralis <span className="text-ink-3/50">›</span>
      </p>
      <span className="sr-only">Astralis is composing a reply</span>
      <div className="max-w-reading space-y-2.5" aria-hidden>
        {["94%", "100%", "72%"].map((w, i) => (
          <div key={i} className="skel h-[.625rem]" style={{ width: w, animationDelay: `${i * 160}ms` }} />
        ))}
      </div>
      <p className="mt-3.5 font-mono text-[.625rem] uppercase tracking-[.12em] text-ink-3">
        working on a reply
      </p>
    </div>
  );
}
