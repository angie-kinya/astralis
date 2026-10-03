import { paragraphs } from "@/lib/format";

/**
 * Design ethics live here.
 *
 * This is not an error state and must not look like one: no red, no alarm
 * icon, no shake. Someone who has just written the hardest sentence of their
 * week should feel handed a card by a person who cares, not caught by a filter.
 *
 * The label says plainly that this is a standard response, which is true: the
 * model was never called. It deliberately avoids words like "flagged" or
 * "crisis detected" — the person reading it should feel met, not processed.
 */
export default function CrisisNotice({ text }: { text: string }) {
  return (
    <section role="note" aria-label="Support resources"
             className="care animate-rise rounded-xl border border-amber/30 bg-amber/[.06] p-5 sm:p-6">
      <p className="font-mono text-[.625rem] uppercase tracking-[.14em] text-amber">
        a standard response for moments like this
      </p>

      <div className="reply mt-4">
        {paragraphs(text).map((p, i) => (
          <p key={i} className="whitespace-pre-wrap">{renderBold(p)}</p>
        ))}
      </div>

      <div className="mt-6 flex flex-wrap gap-2">
        <a href="tel:+254722178177" className="btn-quiet">Call Befrienders Kenya</a>
        <a href="tel:1199" className="btn-quiet">Kenya Red Cross · 1199</a>
        <a href="https://findahelpline.com" target="_blank" rel="noreferrer noopener"
           className="btn-quiet">Find a helpline near you</a>
      </div>
    </section>
  );
}

/** Bold only. No HTML is ever constructed from model or user text. */
function renderBold(text: string) {
  return text.split(/(\*\*[^*]+\*\*)/g).map((part, i) =>
    part.startsWith("**") && part.endsWith("**")
      ? <strong key={i} className="font-semibold">{part.slice(2, -2)}</strong>
      : <span key={i}>{part}</span>,
  );
}
