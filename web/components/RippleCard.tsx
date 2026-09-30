"use client";

import { useEffect, useId, useRef, useState } from "react";

/**
 * A card that opens by revealing a second layer in a circle grown from the
 * exact point it was pressed. The styling lives in globals.css (.ripple-*).
 *
 * The front is not itself a button, because a heading and a paragraph cannot
 * sit inside one. Instead a transparent button is stretched over it, so the
 * whole card is one target while the front text still reads normally to a
 * screen reader.
 *
 * A keyboard press fires a click with detail 0, which opens from the centre.
 * Escape or the close control collapses the circle back to where it opened,
 * and focus returns to the card.
 */
export default function RippleCard({ label, front, back, announce, className = "" }: {
  /** Accessible name for the opening button, e.g. "Show steps for Box breathing". */
  label: string;
  front: React.ReactNode;
  back: (open: boolean) => React.ReactNode;
  /** Read out by screen readers when the card opens. */
  announce: string;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const card = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const closer = useRef<HTMLButtonElement>(null);
  const layerId = useId();

  useEffect(() => { if (open) closer.current?.focus(); }, [open]);

  function reveal(e: React.MouseEvent) {
    const el = card.current;
    if (!el) return;
    if (open) { close(); return; }
    const r = el.getBoundingClientRect();
    const fromKeyboard = e.detail === 0;
    const x = fromKeyboard ? r.width / 2 : e.clientX - r.left;
    const y = fromKeyboard ? r.height / 2 : e.clientY - r.top;
    // The radius that reaches the farthest corner, so the circle always covers the card.
    const radius = Math.hypot(Math.max(x, r.width - x), Math.max(y, r.height - y));
    el.style.setProperty("--x", `${x}px`);
    el.style.setProperty("--y", `${y}px`);
    el.style.setProperty("--r", `${Math.ceil(radius) + 2}px`);
    setOpen(true);
  }

  function close() {
    setOpen(false);
    trigger.current?.focus();
  }

  function hint(e: React.PointerEvent) {
    if (e.pointerType !== "mouse" || !card.current) return;
    const r = card.current.getBoundingClientRect();
    card.current.style.setProperty("--hx", `${e.clientX - r.left}px`);
    card.current.style.setProperty("--hy", `${e.clientY - r.top}px`);
  }

  return (
    <div ref={card} onPointerMove={hint}
         onKeyDown={(e) => { if (open && e.key === "Escape") { e.stopPropagation(); close(); } }}
         className={`card ripple-card ${className}`}>
      <div className="flex flex-col p-4">
        {front}
      </div>

      <button ref={trigger} type="button" onClick={reveal}
              aria-label={label} aria-expanded={open} aria-controls={layerId}
              className="absolute inset-0 rounded-xl focus-visible:ring-inset focus-visible:ring-offset-0">
        {!open && <span aria-hidden className="ripple-hint" />}
      </button>

      <div id={layerId} role="region" aria-label={label} data-open={open}
           className="ripple-layer flex flex-col p-4">
        {/* relative z-10: the back content may pull itself up beside this
            button (a heading with a negative margin, say). Padding is still
            part of an element's box, so without a stacking order that heading
            would sit on top of the button and swallow every click on it. */}
        <button ref={closer} type="button" onClick={close}
                className="relative z-10 self-end rounded font-mono text-[.625rem] uppercase tracking-[.12em]
                           text-ink-3 transition-colors duration-150 ease-swift hover:text-ink">
          close <span aria-hidden>×</span>
        </button>
        {back(open)}
      </div>

      <p aria-live="polite" className="sr-only">{open ? announce : ""}</p>
    </div>
  );
}
