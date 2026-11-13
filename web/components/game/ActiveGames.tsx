"use client";

import Link from "next/link";

import { KindGlyph } from "@/components/game/Glyphs";
import { LEVEL_NAMES, SUDOKU_LEVEL_NAMES } from "@/components/game/rules";
import type { Game } from "@/lib/api";

/** games you can pick back up, every kind, the ones waiting on you first */
export default function ActiveGames({ games }: { games: Game[] }) {
  const sorted = [...games].sort((a, b) => Number(b.myTurn) - Number(a.myTurn) || b.updatedAt.localeCompare(a.updatedAt));
  return (
    <ul className="space-y-1.5">
      {sorted.map((g) => (
        <li key={g.id}>
          <Link href={`/play/${g.id}`}
                className="flex min-h-[3rem] items-center gap-3 rounded-xl border border-line bg-surface/70 px-3 py-2
                           backdrop-blur-[3px] transition-colors duration-150 hover:border-line-2 hover:bg-raised/60">
            <KindGlyph kind={g.kind} className="h-4 w-4 shrink-0 text-violet-2" />
            <span className="min-w-0 flex-1">
              <span className="block truncate font-mono text-[.75rem] text-ink">{title(g)}</span>
              <span className="block truncate font-mono text-[.5625rem] text-ink-3">{line(g)}</span>
            </span>
            <span className={g.myTurn ? "dot-on" : "dot-off"} aria-hidden />
            <span className="font-mono text-[.5625rem] uppercase tracking-[.12em] text-violet-2">resume</span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

function who(g: Game): string {
  if (g.mode === "peer") return `with ${g.peerHandle ?? "someone"}`;
  if (g.mode === "astralis") return "with astralis";
  return g.kind === "constellations" ? "solo" : "";
}

function title(g: Game): string {
  const o = g.options ?? {};
  if (g.kind === "words") {
    const what = [o.day ? "daily" : null, o.topic, o.mode].filter(Boolean).join(" · ");
    return [g.mode === "peer" ? "words duel" : "words", who(g)].filter(Boolean).join(" ") + (what ? ` · ${what}` : "");
  }
  if (g.kind === "sudoku") {
    return `sudoku · ${SUDOKU_LEVEL_NAMES[g.level] ?? `level ${g.level}`}${o.day ? " · daily" : ""}`;
  }
  return `${LEVEL_NAMES[g.level] ?? "constellations"} ${who(g)}`;
}

function line(g: Game): string {
  if (g.kind !== "constellations") return g.myTurn ? "your turn" : g.mode === "peer" ? "waiting for them to finish" : "in progress";
  const s = g.score ?? { me: 0, them: 0 };
  if (g.mode === "solo") return `violet ${s.me} · cyan ${s.them}`;
  const them = g.mode === "astralis" ? "astralis" : "them";
  return `${g.myTurn ? "your move" : "their move"} · you ${s.me} · ${them} ${s.them}`;
}
