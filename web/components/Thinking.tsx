import { AstralisMark, Gutter } from "@/components/Speaker";

/**
 * no skeleton, no dots, no "working on a reply". a skeleton is the loading
 * signature of every llm app, and a phrase that is always shown says nothing.
 *
 * the waiting state is astralis's own mark, breathing slowly in the gutter
 * where its reply will land, so the answer arrives where the eye already is.
 * the only words come from the agent over sse (turn.status), sent only when
 * true and specific: "reading 2 studies" means retrieval actually ran
 */
export default function Thinking({ status }: { status: string | null }) {
  return (
    <div className="flex animate-fade items-start gap-2.5 sm:gap-3" role="status" aria-live="polite">
      <Gutter><AstralisMark breathing /></Gutter>
      <p className="mt-2.5 font-mono text-[.625rem] tracking-[.06em] text-ink-3 sm:mt-3">
        {status && <span key={status} className="animate-fade">{status}</span>}
      </p>
      {/* screen readers still need to know something is happening */}
      {!status && <span className="sr-only">Astralis is replying</span>}
    </div>
  );
}
