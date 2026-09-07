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
 * The deliberate departure from every other chat UI: the assistant does not
 * get a bubble.
 *
 * Bubbles say "instant message". This app wants its replies read slowly, so
 * assistant turns are set as prose at a reading measure with a hairline rule,
 * the way a considered note would be. Only the person's own words are bubbled,
 * because those genuinely are short utterances.
 */
export default function TurnView({ turn }: { turn: Turn }) {
  if (turn.role === "user") {
    return (
      <div className="flex animate-rise justify-end">
        <div className="max-w-measure rounded-xl rounded-br-sm border border-line bg-raised
                        px-4 py-3 text-[0.9375rem] leading-relaxed text-ink">
          <p className="whitespace-pre-wrap">{turn.content}</p>
        </div>
      </div>
    );
  }

  if (turn.crisis) return <CrisisNotice text={turn.content} />;

  return (
    <article className="animate-rise border-l border-line pl-5 sm:pl-6">
      <div className="prose-astralis max-w-reading">
        {paragraphs(turn.content).map((p, i) => (
          <p key={i} className="whitespace-pre-wrap">{renderBold(p)}</p>
        ))}
      </div>

      {!!turn.sources?.length && <SourceList sources={turn.sources} />}
      {turn.recommendationId && <Feedback recommendationId={turn.recommendationId} />}
    </article>
  );
}

/**
 * The model writes plain prose with occasional **bold**. Handling that one
 * case by hand avoids pulling in a markdown renderer — and with it, an
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
