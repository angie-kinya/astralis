import Butterfly from "@/components/Butterflies";

/**
 * no skeleton, no dots, no "working on a reply". a skeleton is the loading
 * signature of every llm app, and a phrase that is always shown says nothing.
 *
 * the only words here come from the agent itself over sse (turn.status), and
 * it sends one only when it is true and specific: "reading 2 studies" means
 * retrieval actually ran. with no status there is just the resting wing.
 */
export default function Thinking({ status }: { status: string | null }) {
  return (
    <div className="animate-fade border-l border-violet/45 pl-4 sm:pl-5" role="status" aria-live="polite">
      <p className="flex items-center gap-2 font-mono text-[.625rem] tracking-[.06em] text-ink-3">
        <Butterfly pose="hover" className="h-3.5 w-3.5 shrink-0 text-violet-2" />
        {status && <span key={status} className="animate-fade">{status}</span>}
      </p>
      {/* screen readers still need to know something is happening */}
      {!status && <span className="sr-only">Astralis is replying</span>}
    </div>
  );
}
