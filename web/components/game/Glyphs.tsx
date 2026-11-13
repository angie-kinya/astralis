import type { GameKind } from "@/lib/api";

/** each game's small sign, drawn in currentColor at the icon weight */

export function ConstellationGlyph({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className}
         fill="none" stroke="currentColor" strokeWidth={1.3} strokeLinecap="round">
      <path d="M6 7h12M18 7v10" opacity=".75" />
      <path d="M6 7v10M6 17h12" strokeDasharray=".01 2.4" opacity=".55" />
      {[[6, 7], [18, 7], [6, 17], [18, 17]].map(([x, y]) => (
        <circle key={`${x}${y}`} cx={x} cy={y} r={1.5} fill="currentColor" stroke="none" />
      ))}
    </svg>
  );
}

/** three letter tiles, the middle one settled */
export function WordsGlyph({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className}
         fill="none" stroke="currentColor" strokeWidth={1.3} strokeLinejoin="round">
      <rect x="2.5" y="8.5" width="5.5" height="7" rx="1.2" opacity=".55" />
      <rect x="9.25" y="8.5" width="5.5" height="7" rx="1.2" fill="currentColor" fillOpacity=".22" />
      <rect x="16" y="8.5" width="5.5" height="7" rx="1.2" opacity=".55" />
    </svg>
  );
}

/** a small grid with one box drawn heavier */
export function SudokuGlyph({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={className}
         fill="none" stroke="currentColor" strokeWidth={1.3} strokeLinecap="round">
      <rect x="4" y="4" width="16" height="16" rx="2" />
      <path d="M9.33 4v16M14.67 4v16M4 9.33h16M4 14.67h16" opacity=".45" />
      <rect x="9.33" y="9.33" width="5.33" height="5.33" fill="currentColor" fillOpacity=".22" stroke="none" />
    </svg>
  );
}

export function KindGlyph({ kind, className }: { kind: GameKind; className?: string }) {
  if (kind === "words") return <WordsGlyph className={className} />;
  if (kind === "sudoku") return <SudokuGlyph className={className} />;
  return <ConstellationGlyph className={className} />;
}
