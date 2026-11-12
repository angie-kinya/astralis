"use client";

import { useRef, useState } from "react";

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
 * no speaker labels. in a one-to-one chat the speaker is obvious, and a mono
 * `astralis ›` reads like a terminal prompt, i.e. a tool. the left border
 * colour is the only cue: violet for astralis, line for you.
 *
 * an assistant reply is rendered as short bursts, one block per paragraph,
 * all arriving together. assistants answer in slabs; people send bursts. no
 * fake stagger: the reply is already written, pretending otherwise is theatre.
 */
export default function TurnView({ turn }: { turn: Turn }) {
  const [revealed, setRevealed] = useState(false);
  const press = useRef<ReturnType<typeof setTimeout>>();

  if (turn.crisis) return <CrisisNotice text={turn.content} />;

  if (turn.role === "user") {
    return (
      <article className="animate-rise border-l border-line pl-4 sm:pl-5">
        <p className="max-w-reading whitespace-pre-wrap font-sans text-[.9375rem]
                      leading-[1.7] text-ink-2">{turn.content}</p>
      </article>
    );
  }

  // long-press reveals the rating on touch, where there is no hover
  const touch = {
    onTouchStart: () => { press.current = setTimeout(() => setRevealed(true), 450); },
    onTouchEnd:   () => clearTimeout(press.current),
    onTouchMove:  () => clearTimeout(press.current),
  };

  return (
    <article className="group animate-rise space-y-2" {...touch}>
      {paragraphs(turn.content).map((p, i) => (
        <div key={i} className="reply max-w-reading border-l border-violet/45 pl-4 sm:pl-5">
          <p className="whitespace-pre-wrap">{renderBold(p)}</p>
        </div>
      ))}

      {!!turn.sources?.length && (
        <div className="pl-4 sm:pl-5"><SourceList sources={turn.sources} /></div>
      )}
      {turn.recommendationId && (
        <div className="pl-4 sm:pl-5">
          <Feedback recommendationId={turn.recommendationId} revealed={revealed} />
        </div>
      )}
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
