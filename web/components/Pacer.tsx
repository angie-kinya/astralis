"use client";

import { useEffect, useState } from "react";

import Butterfly from "@/components/Butterflies";
import type { PacePhase } from "@/lib/resources";
import { useReducedMotion } from "@/lib/motion";

const REST = 0.45;

/**
 * A breathing pacer: a ring that grows on the inhale, holds, and shrinks on
 * the exhale, timed to the practice. It never starts by itself, and it stops
 * as soon as the card closes. With reduced motion the ring is replaced by the
 * phase name and a countdown.
 */
export default function Pacer({ pace, active }: { pace: PacePhase[]; active: boolean }) {
  const reduced = useReducedMotion();
  const [running, setRunning] = useState(false);
  const [step, setStep] = useState({ idx: 0, left: pace[0]!.seconds });

  useEffect(() => { if (!active) setRunning(false); }, [active]);

  useEffect(() => {
    if (!running) { setStep({ idx: 0, left: pace[0]!.seconds }); return; }
    const id = setInterval(() => {
      setStep(({ idx, left }) => {
        if (left > 1) return { idx, left: left - 1 };
        const next = (idx + 1) % pace.length;
        return { idx: next, left: pace[next]!.seconds };
      });
    }, 1000);
    return () => clearInterval(id);
  }, [running, pace]);

  const phase = pace[step.idx]!;

  return (
    <div className="mt-4 flex flex-col items-center gap-3">
      {reduced ? (
        <p className="font-mono text-[2rem] tabular-nums leading-none text-violet-2" aria-hidden>
          {running ? step.left : "·"}
        </p>
      ) : (
        <div className="relative grid h-24 w-24 place-items-center" aria-hidden>
          <div className="pacer-ring"
               style={{
                 transform: `scale(${running ? phase.scale : REST})`,
                 transitionDuration: running ? `${phase.seconds}s` : "300ms",
               }} />
          {/* The same breath, read as a wing: open on the inhale, folded on
              the exhale, on exactly the pacer's clock. The butterfly is what
              the exercise is *for* — a quiet body settling, not a progress bar. */}
          <span className="absolute inset-0 grid place-items-center">
            <Butterfly pose="still" className="h-7 w-7 text-violet-2"
                       style={{
                         transform: `scaleX(${running ? phase.scale : 0.72})`,
                         transition: `transform ${running ? phase.seconds : 400}ms ease-in-out`,
                       }} />
          </span>
        </div>
      )}
      <p className="font-sans text-[.9375rem] font-medium text-ink" role="status">
        {running ? phase.label : "Press Start when you are ready"}
      </p>
      <button type="button" onClick={() => setRunning((r) => !r)} className="btn-quiet !py-1.5">
        {running ? "Stop" : "Start"}
      </button>
    </div>
  );
}
