"use client";

import Link from "next/link";

import { useMe } from "@/components/AppGate";
import GameView from "@/components/game/GameView";

export default function PlayPage({ params }: { params: { gameId: string } }) {
  const { me } = useMe();
  // the api answers 404 here too when the flag is off; this just says so kindly
  if (!me.games) {
    return (
      <div className="grid h-full place-items-center px-6 text-center">
        <p className="font-mono text-[.75rem] text-ink-3">
          nothing to play here. <Link href="/circle" className="text-ink-2 underline underline-offset-2">back to the circle</Link>
        </p>
      </div>
    );
  }
  return <GameView key={params.gameId} gameId={params.gameId} />;
}
