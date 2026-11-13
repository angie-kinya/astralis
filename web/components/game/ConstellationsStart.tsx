"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import LevelPicker from "@/components/game/LevelPicker";
import { useCatalog, useProgress } from "@/components/game/useCatalog";
import { ApiError, createGame, userMessage } from "@/lib/api";

/**
 * a game with astralis or on your own, at any sky you have open. games with a
 * person start from their thread, never from here, so nobody is cold-invited
 */
export default function ConstellationsStart() {
  const router = useRouter();
  const cat = useCatalog();
  const { unlocked, refresh } = useProgress();
  const [level, setLevel] = useState<number | null>(null);
  const [starting, setStarting] = useState<"astralis" | "solo" | null>(null);
  const [error, setError] = useState<string | null>(null);

  // start on the newest open sky; a choice made here is kept
  useEffect(() => { if (unlocked !== null && level === null) setLevel(unlocked); }, [unlocked, level]);

  async function start(mode: "astralis" | "solo") {
    setStarting(mode); setError(null);
    try {
      const { game } = await createGame("constellations", mode, { level: level ?? 1 });
      router.push(`/play/${game.id}`);
    } catch (e) {
      if (e instanceof ApiError && e.code === "level_locked") {
        setError("that sky is not open yet.");
        refresh();
      } else setError(userMessage(e));
      setStarting(null);
    }
  }

  return (
    <div>
      {unlocked === null
        ? <div className="skel h-2 w-40" />
        : <LevelPicker levels={cat?.constellations.levels} unlocked={unlocked}
                       value={level ?? unlocked} onChange={setLevel} />}
      <div className="mt-3 flex flex-wrap gap-2">
        <button className="btn-primary !px-3 !py-2" disabled={!!starting || unlocked === null}
                onClick={() => void start("astralis")}>
          {starting === "astralis" ? "Starting…" : "Play Astralis"}
        </button>
        <button className="btn-quiet !px-3 !py-2" disabled={!!starting || unlocked === null}
                onClick={() => void start("solo")}>
          {starting === "solo" ? "Starting…" : "Play solo"}
        </button>
      </div>
      <p className="mt-2 font-mono text-[.5625rem] leading-relaxed text-ink-3">
        to play someone, open your chat with them in the circle and press play.
      </p>
      {error && <p role="alert" className="mt-2 font-mono text-[.6875rem] text-amber">{error}</p>}
    </div>
  );
}
