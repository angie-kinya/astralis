import type { CSSProperties } from "react";

export type Mark = "correct" | "present" | "absent";

// --g-teal comes from the games block in globals.css. correct is a solid teal fill, present
// a warm tint with a firm edge, absent recedes into the pane. none of them is ever a dark
// slab on a dark ground
export const TEAL = "rgb(var(--g-teal))";

export const markStyle: Record<Mark, CSSProperties> = {
  correct: { background: TEAL, borderColor: TEAL },
  present: { background: "rgb(var(--amber) / .2)", borderColor: "rgb(var(--amber) / .75)" },
  absent:  { background: "rgb(var(--raised))", borderColor: "rgb(var(--line))" },
};

export const markText: Record<Mark, string> = {
  // dark ink on the bright dark-theme teal, white on the deeper light-theme one
  correct: "text-g-teal-ink",
  present: "text-ink",
  absent:  "text-ink-3",
};

export const markWord: Record<Mark, string> = {
  correct: "in the right place",
  present: "in the word, elsewhere",
  absent:  "not in the word",
};

/** the gentle nudge for a wrong-length guess. web animations, so no keyframes are needed in globals.css */
export function shake(el: HTMLElement | null, reduced: boolean) {
  if (!el || reduced || typeof el.animate !== "function") return;
  el.animate(
    [
      { transform: "translateX(0)" }, { transform: "translateX(-5px)" }, { transform: "translateX(4px)" },
      { transform: "translateX(-2px)" }, { transform: "translateX(0)" },
    ],
    { duration: 320, easing: "ease-out" },
  );
}
