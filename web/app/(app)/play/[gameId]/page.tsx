"use client";

import Link from "next/link";

import { useMe } from "@/components/AppGate";
import PlayRoom from "@/components/game/PlayRoom";

export default function PlayGamePage({ params }: { params: { gameId: string } }) {
  const { me } = useMe();
  // the api answers 404 here too when the flag is off; this just says so kindly
  if (!me.games) {
    return (
      <div className="grid h-full place-items-center px-6 text-center">
        <p className="font-mono text-[.75rem] text-ink-3">
          nothing to play here. <Link href="/chat" className="text-ink-2 underline underline-offset-2">back to astralis</Link>
        </p>
      </div>
    );
  }
  return <PlayRoom key={params.gameId} gameId={params.gameId} />;
}
