import type { Config } from "tailwindcss";

/**
 * Every colour in the app is a CSS custom property defined in globals.css and
 * re-declared for the light theme. Nothing here is a literal hex, and no
 * component should ever contain one either -- that is the only way a second
 * theme stays honest instead of drifting.
 *
 * The values are space-separated RGB channels so Tailwind's opacity modifiers
 * (bg-surface/70) still work.
 */
const token = (name: string) => `rgb(var(--${name}) / <alpha-value>)`;

export default {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ground:    token("ground"),      // page
        surface:   token("surface"),     // cards, rail
        raised:    token("raised"),      // hover, active
        line:      token("line"),        // hairline borders
        "line-strong": token("line-strong"),
        ink:       token("ink"),         // primary text
        "ink-2":   token("ink-2"),       // secondary text
        "ink-3":   token("ink-3"),       // metadata, disabled
        accent:    token("accent"),      // one accent, used sparingly
        "accent-ink": token("accent-ink"),
        signal:    token("signal"),      // the crisis card only
      },
      fontFamily: {
        // Serif for what the agent writes -- it should read as considered
        // prose, not as machine output. Sans for chrome. Mono for metadata.
        serif: ["var(--font-serif)", "Georgia", "serif"],
        sans:  ["var(--font-sans)", "system-ui", "sans-serif"],
        mono:  ["var(--font-mono)", "ui-monospace", "monospace"],
      },
      maxWidth: {
        // ~62 characters at the body size. Longer lines measurably hurt
        // comprehension, and this app exists to be read carefully.
        measure: "34rem",
        reading: "46rem",
      },
      keyframes: {
        rise: {
          from: { opacity: "0", transform: "translateY(6px)" },
          to:   { opacity: "1", transform: "none" },
        },
        shimmer: {
          "0%, 100%": { opacity: "0.35" },
          "50%":      { opacity: "0.8" },
        },
      },
      animation: {
        rise: "rise 240ms cubic-bezier(0.2, 0.6, 0.2, 1) both",
        shimmer: "shimmer 1.6s ease-in-out infinite",
      },
    },
  },
  plugins: [],
} satisfies Config;
