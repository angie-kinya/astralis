"use client";

import { useEffect, useRef, useState } from "react";

import { IconSend } from "@/components/icons";

/**
 * The composer carries the editor metaphor: a mono caret, mono input, and a
 * hint line that reads like a status bar. It is the control people touch most,
 * so it is the one that most needs to feel like an instrument.
 */
export default function Composer({
  onSend, disabled, placeholder = "What has work been like lately?", max = 4000,
}: {
  onSend: (text: string) => void;
  disabled: boolean;
  placeholder?: string;
  max?: number;
}) {
  const [draft, setDraft] = useState("");
  const ref = useRef<HTMLTextAreaElement>(null);

  // Grow with content up to a ceiling, then scroll. A fixed-height box
  // punishes exactly the long, careful messages this app wants to receive.
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "0px";
    el.style.height = `${Math.min(el.scrollHeight, 220)}px`;
  }, [draft]);

  function submit() {
    const text = draft.trim();
    if (!text || disabled) return;
    setDraft("");
    onSend(text);
  }

  const left = max - draft.length;

  return (
    <div className="rounded-xl border border-line bg-surface transition-colors duration-150
                    ease-swift focus-within:border-violet/55">
      <div className="flex gap-2.5 px-3.5 pt-3">
        <span className="caret mt-[.15rem] select-none text-[.9375rem]" aria-hidden>›</span>
        <textarea
          ref={ref}
          value={draft}
          onChange={(e) => setDraft(e.target.value.slice(0, max))}
          onKeyDown={(e) => {
            // Enter sends; Shift+Enter is a newline. People writing three
            // paragraphs about their job need the newline to be easy.
            if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); submit(); }
          }}
          rows={1}
          disabled={disabled}
          aria-label="Your message"
          placeholder={placeholder}
          className="min-w-0 flex-1 resize-none bg-transparent font-sans text-[.9375rem]
                     leading-[1.7] text-ink outline-none placeholder:text-ink-3/70
                     disabled:opacity-50"
        />
      </div>

      <div className="flex items-center justify-between gap-3 px-3 pb-2.5 pt-1.5">
        <p className="font-mono text-[.625rem] text-ink-3">
          {left < 300
            ? <span className={left < 50 ? "text-amber" : ""}>{left} characters left</span>
            : <>enter to send · shift+enter for a new line</>}
        </p>
        <button onClick={submit} disabled={disabled || !draft.trim()}
                aria-label="Send"
                className="btn-primary !gap-1.5 !px-3 !py-1.5">
          <IconSend className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}
