import type { Config } from "tailwindcss";

/**
 * Two typefaces, deliberately.
 *
 *   mono  — JetBrains Mono. The editor face. It carries every structural
 *           element: handles, timestamps, labels, status, the composer caret.
 *           It is what makes Astralis feel like an instrument rather than a
 *           chat window.
 *   sans  — Sora. Geometric, slightly futuristic, and calm enough to read a
 *           long reply in. It carries prose and headings only.
 *
 * Nothing else. A third face would dilute both.
 *
 * Every colour is a CSS custom property declared in globals.css and redeclared
 * for the light theme. No component contains a hex value — that is the only
 * way a second theme stays honest instead of drifting out of sync.
 */
const token = (name: string) => `rgb(var(--${name}) / <alpha-value>)`;

export default {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg:        token("bg"),
        surface:   token("surface"),
        raised:    token("raised"),
        line:      token("line"),
        "line-2":  token("line-2"),
        ink:       token("ink"),
        "ink-2":   token("ink-2"),
        "ink-3":   token("ink-3"),
        // The primary. Used for Astralis itself, focus, and primary action.
        violet:    token("violet"),
        "violet-2":token("violet-2"),
        // Presence and liveness only. Never decorative.
        cyan:      token("cyan"),
        // Care, caution, and the crisis card. Warm, never alarming red.
        amber:     token("amber"),
        // the play section's accents, inside the games only: correct, selection, a mistake
        "g-teal":    token("g-teal"),
        "g-teal-ink":token("g-teal-ink"),
        "g-sky":     token("g-sky"),
        "g-rose":    token("g-rose"),
      },
      fontFamily: {
        mono: ["var(--font-mono)", "ui-monospace", "monospace"],
        sans: ["var(--font-sans)", "system-ui", "sans-serif"],
      },
      maxWidth: {
        // ~62 characters at the reply size. Longer lines measurably hurt
        // comprehension, and this app exists to be read carefully.
        measure: "34rem",
        reading: "44rem",
      },
      keyframes: {
        rise:   { from: { opacity: "0", transform: "translateY(8px)" },
                  to:   { opacity: "1", transform: "none" } },
        fade:   { from: { opacity: "0" }, to: { opacity: "1" } },
        pulse2: { "0%,100%": { opacity: "1", transform: "scale(1)" },
                  "50%":     { opacity: ".45", transform: "scale(.82)" } },
        sweep:  { "0%": { backgroundPosition: "-160% 0" },
                  "100%": { backgroundPosition: "260% 0" } },
        caret:  { "0%,45%": { opacity: "1" }, "55%,100%": { opacity: "0" } },
      },
      animation: {
        rise:   "rise 240ms cubic-bezier(.2,.6,.2,1) both",
        fade:   "fade 180ms ease-out both",
        pulse2: "pulse2 2.4s ease-in-out infinite",
        sweep:  "sweep 2.1s ease-in-out infinite",
        caret:  "caret 1.1s step-end infinite",
      },
      transitionTimingFunction: { swift: "cubic-bezier(.2,.6,.2,1)" },
    },
  },
  plugins: [],
} satisfies Config;
