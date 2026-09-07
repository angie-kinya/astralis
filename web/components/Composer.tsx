"use client";

import { useEffect, useRef, useState } from "react";

const MAX = 4000; // must match the `max=4000` binding on the Go handler

export default function Composer({
  onSend, disabled,
}: { onSend: (text: string) => void; disabled: boolean }) {
  const [draft, setDraft] = useState("");
  const ref = useRef<HTMLTextAreaElement>(null);

  // Grow with the content up to a ceiling, then scroll. Fixed-height boxes
  // punish exactly the long, careful messages this app wants to receive.
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "0px";
    el.style.height = `${Math.min(el.scrollHeight, 240)}px`;
  }, [draft]);

  function submit() {
    const text = draft.trim();
    if (!text || disabled) return;
    setDraft("");
    onSend(text);
  }

  const remaining = MAX - draft.length;

  return (
    <div className="rounded-xl border border-line bg-surface transition-colors
                    focus-within:border-line-strong">
      <textarea
        ref={ref}
        value={draft}
        onChange={(e) => setDraft(e.target.value.slice(0, MAX))}
        onKeyDown={(e) => {
          // Enter sends; Shift+Enter is a newline. People writing three
          // paragraphs about their job need the newline to be easy.
          if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); submit(); }
        }}
        rows={1}
        disabled={disabled}
        aria-label="Your message"
        placeholder="What has work been like lately?"
        className="block w-full resize-none bg-transparent px-4 pb-2 pt-3.5 text-[0.9375rem]
                   leading-relaxed text-ink outline-none placeholder:text-ink-3
                   disabled:opacity-50"
      />

      <div className="flex items-center justify-between gap-3 px-3 pb-3 pt-1">
        <p className="font-mono text-[0.6875rem] text-ink-3">
          {remaining < 300
            ? <span className={remaining < 50 ? "text-signal" : ""}>{remaining} left</span>
            : <><kbd className="font-sans">Enter</kbd> to send · <kbd className="font-sans">Shift</kbd>+<kbd className="font-sans">Enter</kbd> for a new line</>}
        </p>
        <button className="btn-primary !px-3.5 !py-2" onClick={submit}
                disabled={disabled || !draft.trim()}>
          Send
        </button>
      </div>
    </div>
  );
}
