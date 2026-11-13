"use client";

import { useCallback, useEffect, useState } from "react";

import { type Catalog, type Progress, catalog, progress } from "@/lib/api";

// the catalog only changes with a deploy, so one fetch per tab is plenty
let cached: Promise<Catalog> | null = null;

export function useCatalog(): Catalog | null {
  const [c, setC] = useState<Catalog | null>(null);
  useEffect(() => {
    let gone = false;
    cached ??= catalog().catch((e) => { cached = null; throw e; });
    cached.then((v) => { if (!gone) setC(v); }).catch(() => { /* pickers fall back to level 1 and the names in rules.ts */ });
    return () => { gone = true; };
  }, []);
  return c;
}

/** progress moves when a game finishes, so it is fetched fresh, with a refresh for after a finish */
export function useProgress(): { unlocked: number | null; refresh: () => void } {
  const [unlocked, setUnlocked] = useState<number | null>(null);
  const refresh = useCallback(() => {
    progress().then((p: Progress) => setUnlocked(p.constellations.unlocked)).catch(() => setUnlocked((u) => u ?? 1));
  }, []);
  useEffect(() => { refresh(); }, [refresh]);
  return { unlocked, refresh };
}
