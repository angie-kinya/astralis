"use client";

import { useEffect, useRef, useState } from "react";

import { useReducedMotion } from "@/lib/motion";

/**
 * A very large single-stroke wing outline at a page edge, partly off-screen,
 * barely visible. The two shapes are different wings, so the landing page's
 * pair suggests one butterfly without drawing it literally.
 *
 * It draws itself once when it first comes into view (stroke-dashoffset over
 * 5s), then settles to its resting opacity and is completely still.
 * Reduced motion: shown at rest, never drawn.
 */
const SHAPES = {
  // A forewing sweeping up to a rounded apex, then a hindwing with a tail lobe.
  right: {
    viewBox: "0 0 700 900",
    d: "M60 470C130 360 250 190 430 90C520 40 640 30 690 80C730 120 700 230 640 300C580 370 480 420 380 440C300 456 180 462 60 474C170 490 320 520 420 590C500 646 540 730 500 800C470 850 400 860 350 830C344 852 334 870 322 880C312 846 302 818 290 796C220 720 130 600 60 480",
  },
  // A rounder forewing over a broad, two-lobed hindwing.
  left: {
    viewBox: "0 0 600 800",
    d: "M560 300C480 220 360 120 220 90C140 74 70 100 60 160C50 220 110 280 200 310C300 340 430 330 560 312C460 350 330 420 250 520C190 600 190 690 250 730C290 756 340 740 370 700C390 730 430 740 460 710C520 640 550 470 560 320",
  },
} as const;

const DRAW_MS = 5000;

export default function WingContour({ side, className = "" }: {
  side: keyof typeof SHAPES; className?: string;
}) {
  const reduced = useReducedMotion();
  const ref = useRef<SVGSVGElement>(null);
  const [phase, setPhase] = useState<"idle" | "drawing" | "resting">("idle");

  useEffect(() => {
    const el = ref.current;
    if (!el || phase !== "idle") return;
    if (reduced || matchMedia("(prefers-reduced-motion: reduce)").matches || !("IntersectionObserver" in window)) {
      setPhase("resting");
      return;
    }
    const io = new IntersectionObserver(([e]) => {
      if (!e?.isIntersecting) return;
      io.disconnect();
      setPhase("drawing");
    }, { threshold: .15 });
    io.observe(el);
    return () => io.disconnect();
  }, [reduced, phase]);

  useEffect(() => {
    if (phase !== "drawing") return;
    const t = window.setTimeout(() => setPhase("resting"), DRAW_MS + 300);
    return () => clearTimeout(t);
  }, [phase]);

  const { viewBox, d } = SHAPES[side];
  return (
    <svg ref={ref} viewBox={viewBox} aria-hidden data-phase={phase}
         className={`wing-contour pointer-events-none absolute -z-10 ${className}`}>
      <path d={d} pathLength={1} />
    </svg>
  );
}
