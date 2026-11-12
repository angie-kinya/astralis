"use client";

import { useEffect, useRef, useState } from "react";

import type { ReportReason } from "@/lib/api";

const REASONS: [ReportReason, string][] = [
  ["harassment", "Harassment or abuse"],
  ["spam", "Spam or advertising"],
  ["unsafe", "Something that worries you about their safety or yours"],
  ["other", "Something else"],
];

/**
 * Reporting a person, or one of their messages. The server keeps a snapshot
 * of the message, so the sender deleting it afterwards cannot erase what was
 * reported. The reported person is never told.
 */
export default function ReportSheet({ handle, withMessage, onSubmit, onClose }: {
  handle: string;
  withMessage: boolean;
  onSubmit: (reason: ReportReason) => Promise<void>;
  onClose: () => void;
}) {
  const [reason, setReason] = useState<ReportReason | null>(null);
  const [busy, setBusy] = useState(false);
  const first = useRef<HTMLInputElement>(null);

  useEffect(() => {
    first.current?.focus();
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", esc);
    return () => window.removeEventListener("keydown", esc);
  }, [onClose]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!reason) return;
    setBusy(true);
    try { await onSubmit(reason); } finally { setBusy(false); }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-bg/70 px-4 backdrop-blur-sm"
         onClick={onClose}>
      <form role="dialog" aria-modal="true" aria-labelledby="report-title" onSubmit={submit}
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-sm rounded-xl border border-line bg-surface p-5 shadow-xl">
        <p id="report-title" className="label">
          report {withMessage ? "this message" : handle}
        </p>
        <p className="mt-2 text-[.8125rem] leading-relaxed text-ink-2">
          {handle} will not be told. Someone looks at every report.
        </p>

        <fieldset className="mt-4 space-y-1.5">
          <legend className="sr-only">Reason</legend>
          {REASONS.map(([value, text], i) => (
            <label key={value}
                   className={`flex cursor-pointer items-start gap-2.5 rounded-lg px-2.5 py-2
                               text-[.8125rem] leading-snug transition-colors duration-150
                               ${reason === value ? "bg-raised text-ink" : "text-ink-2 hover:bg-raised/60"}`}>
              <input ref={i === 0 ? first : undefined} type="radio" name="reason" value={value}
                     checked={reason === value} onChange={() => setReason(value)}
                     className="mt-[.2em] accent-violet" />
              {text}
            </label>
          ))}
        </fieldset>

        <div className="mt-5 flex justify-end gap-2">
          <button type="button" className="btn-quiet" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn-danger" disabled={!reason || busy}>
            {busy ? "Sending…" : "Report"}
          </button>
        </div>
      </form>
    </div>
  );
}
