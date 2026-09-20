"use client";

import CrisisNotice from "@/components/CrisisNotice";
import Feedback from "@/components/Feedback";
import SourceList from "@/components/SourceList";
import type { Source } from "@/lib/api";
import { paragraphs } from "@/lib/format";

export type Turn = {
  key: string;
  role: "user" | "assistant";
  content: string;
  crisis?: boolean;
  recommendationId?: string;
  sources?: Source[];
};

/**
 * The deliberate departure from every other chat UI: nobody gets a bubble.
 *
 * Instead each turn is attributed by a mono label — `you ›` / `astralis ›` —
 * and the content sits under it at a reading measure. It reads like a
 * transcript or a log, which is both more honest about what this is and far
 * better suited to replies that are meant to be read slowly rather than
 * skimmed and answered in two seconds.
 */
export default function TurnView({ turn, handle }: { turn: Turn; handle: string }) {
  if (turn.crisis) return <CrisisNotice text={turn.content} />;

  const mine = turn.role === "user";

  return (
    <article className={`animate-rise border-l pl-4 sm:pl-5 ${mine ? "border-line" : "border-violet/45"}`}>
      <p className="mb-2 font-mono text-[.625rem] uppercase tracking-[.14em]">
        <span className={mine ? "text-ink-3" : "text-violet-2"}>{mine ? handle : "astralis"}</span>
        <span className="text-ink-3/50"> ›</span>
      </p>

      {mine ? (
        <p className="max-w-reading whitespace-pre-wrap font-sans text-[.9375rem]
                      leading-[1.7] text-ink-2">{turn.content}</p>
      ) : (
        <div className="reply max-w-reading">
          {paragraphs(turn.content).map((p, i) => (
            <p key={i} className="whitespace-pre-wrap">{renderBold(p)}</p>
          ))}
        </div>
      )}

      {!mine && !!turn.sources?.length && <SourceList sources={turn.sources} />}
      {!mine && turn.recommendationId && <Feedback recommendationId={turn.recommendationId} />}
    </article>
  );
}

/**
 * The model writes plain prose with occasional **bold**. Handling that one
 * case by hand avoids pulling in a markdown renderer, and with it a
 * dangerouslySetInnerHTML call sitting directly downstream of an LLM. Model
 * output is untrusted input; it never becomes HTML here.
 */
function renderBold(text: string) {
  return text.split(/(\*\*[^*]+\*\*)/g).map((part, i) =>
    part.startsWith("**") && part.endsWith("**")
      ? <strong key={i}>{part.slice(2, -2)}</strong>
      : <span key={i}>{part}</span>,
  );
}
