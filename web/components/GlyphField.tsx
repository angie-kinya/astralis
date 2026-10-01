"use client";

import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";

import { GLYPH_BODY, GLYPH_HALF, MIRROR } from "@/components/Butterflies";
import { useReducedMotion } from "@/lib/motion";

/**
 * A sparse field of tiny butterfly outlines, fixed behind everything. Texture,
 * not animation: nothing flies. Each glyph independently fades in with a soft
 * pop (scale .92 -> 1), holds, fades out, stays gone a while, and comes back
 * somewhere else. At most two are changing at any moment, so the field never
 * blinks in unison.
 *
 * Cost: timers and CSS transitions only, no animation loop and no canvas.
 * Timers pause while the tab is hidden. A glyph is only ever moved while it
 * is invisible.
 *
 * Text wins: anything marked data-butterfly-clear (the chat's reading column
 * and composer; the sign-in, onboarding and consent content; the landing's
 * main and footer) is a keep-out zone. Zones are read at each
 * placement, and every few seconds a visible glyph that a zone has moved onto
 * (a route change, a resize) is faded out early.
 *
 * Reduced motion: the same field, placed once, at rest. No fading.
 */

type Glyph = {
  wrap: SVGGElement; use: SVGUseElement;
  rest: number; size: number; cx: number; cy: number;
  placed: boolean; visible: boolean; leaving: boolean;
  gen: number;                 // bumped on every fade-out, so stale timers do nothing
};
type Zone = { l: number; t: number; r: number; b: number };
type Timer = { fn: () => void; left: number; at: number; id: number };

const NS = "http://www.w3.org/2000/svg";
const rand = (a: number, b: number) => a + Math.random() * (b - a);
const MAX_BUSY = 2;           // glyphs fading in or out at once
const AREA_PER_GLYPH = 85_000;
const ZONE_PAD = 18;
const ZONE_CHECK_MS = 2500;

export default function GlyphField({ density = 1, startAtRest = false }: {
  /** Multiplier on the base density: 0.8 for sign-in and onboarding, 0.55 in the app. */
  density?: number;
  /** Open with about two thirds already visible (only where the layout is ready at mount). */
  startAtRest?: boolean;
}) {
  const reduced = useReducedMotion();
  const svgRef = useRef<SVGSVGElement>(null);
  const checkRef = useRef<() => void>(() => {});
  const path = usePathname();

  // A new route can put a reading column where a glyph is resting.
  useEffect(() => { checkRef.current(); }, [path]);

  useEffect(() => {
    const svg = svgRef.current;
    if (!svg) return;
    const still = reduced || matchMedia("(prefers-reduced-motion: reduce)").matches;

    // ---- pausable timers
    const timers = new Set<Timer>();
    let alive = true;
    function later(fn: () => void, ms: number) {
      const t: Timer = { fn, left: ms, at: performance.now(), id: 0 };
      t.id = window.setTimeout(() => { timers.delete(t); if (alive) fn(); }, ms);
      timers.add(t);
    }
    function onVis() {
      const now = performance.now();
      for (const t of timers) {
        if (document.hidden) { clearTimeout(t.id); t.left -= now - t.at; }
        else {
          t.at = now;
          t.id = window.setTimeout(() => { timers.delete(t); if (alive) t.fn(); }, Math.max(0, t.left));
        }
      }
    }
    document.addEventListener("visibilitychange", onVis);

    let glyphs: Glyph[] = [];
    let busy = 0, W = 0, H = 0, minGap = 0;

    // data-butterfly-clear="content" means "where this element's children
    // are", for full-height columns whose content sits in the middle.
    function zones(): Zone[] {
      return [...document.querySelectorAll<HTMLElement>("[data-butterfly-clear]")].flatMap((e) => {
        const els = e.dataset.butterflyClear === "content" ? [...e.children] : [e];
        let l = Infinity, t = Infinity, r = -Infinity, b = -Infinity;
        for (const c of els) {
          const q = c.getBoundingClientRect();
          if (!q.width || !q.height) continue;
          l = Math.min(l, q.left); t = Math.min(t, q.top); r = Math.max(r, q.right); b = Math.max(b, q.bottom);
        }
        return l < r ? [{ l: l - ZONE_PAD, t: t - ZONE_PAD, r: r + ZONE_PAD, b: b + ZONE_PAD }] : [];
      });
    }
    const inZone = (g: { cx: number; cy: number; size: number }, zs: Zone[]) =>
      zs.some((z) => g.cx > z.l - g.size && g.cx < z.r + g.size && g.cy > z.t - g.size && g.cy < z.b + g.size);

    function place(g: Glyph) {
      const zs = zones();
      for (let i = 0; i < 40; i++) {
        const size = rand(10, 22), x = rand(12, W - 12 - size), y = rand(12, H - 12 - size);
        const c = { cx: x + size / 2, cy: y + size / 2, size };
        if (inZone(c, zs)) continue;
        if (glyphs.some((o) => o !== g && o.placed && Math.hypot(o.cx - c.cx, o.cy - c.cy) < minGap)) continue;
        Object.assign(g, c, { placed: true });
        g.wrap.setAttribute("transform",
          `translate(${x.toFixed(1)} ${y.toFixed(1)}) rotate(${rand(-25, 25).toFixed(1)} ${size / 2} ${size / 2})`);
        g.use.setAttribute("width", String(size));
        g.use.setAttribute("height", String(size));
        return true;
      }
      return false;
    }

    const style = (g: Glyph, transition: string, opacity: number, transform: string) => {
      g.use.style.transition = transition;
      g.use.style.opacity = String(opacity);
      g.use.style.transform = transform;
    };

    function whenFree(fn: () => void) {
      if (busy < MAX_BUSY) { busy++; fn(); }
      else later(() => whenFree(fn), rand(1200, 2600));
    }

    function fadeIn(g: Glyph) {
      const d = rand(2500, 4000);
      style(g, "none", 0, "scale(.92)");
      g.use.getBoundingClientRect();          // commit the start state
      g.visible = true;
      const gen = g.gen;
      style(g, `opacity ${d}ms cubic-bezier(.16,1,.3,1), transform ${d}ms cubic-bezier(.16,1,.3,1)`, g.rest, "scale(1)");
      later(() => { busy--; if (g.gen === gen) hold(g); }, d);
    }
    function hold(g: Glyph) {
      if (g.leaving) return;
      const gen = g.gen, h = rand(8000, 20000), a = rand(0, Math.PI * 2), d = rand(1, 2);
      style(g, `transform ${h}ms linear`, g.rest,
        `translate(${(Math.cos(a) * d).toFixed(2)}px, ${(Math.sin(a) * d).toFixed(2)}px)`);
      later(() => { if (g.gen === gen) whenFree(() => (g.gen === gen ? fadeOut(g, rand(3000, 5000)) : busy--)); }, h);
    }
    function fadeOut(g: Glyph, d: number, counted = true) {
      g.leaving = true;
      g.gen++;
      g.use.style.transition = `opacity ${d}ms ease-in-out`;
      g.use.style.opacity = "0";
      later(() => {
        if (counted) busy--;
        g.placed = g.visible = g.leaving = false;
        gone(g);
      }, d);
    }
    function gone(g: Glyph) {
      later(() => {
        if (!place(g)) return gone(g);        // no room right now: stay gone longer
        whenFree(() => fadeIn(g));
      }, rand(6000, 18000));
    }

    // Legibility beats the "two at a time" rule: a glyph a zone has moved
    // onto leaves at once, quickly.
    function check() {
      const zs = zones();
      for (const g of glyphs) {
        if (!g.visible || g.leaving || !inZone(g, zs)) continue;
        if (still) { g.use.style.opacity = "0"; g.visible = g.placed = false; if (place(g)) { g.visible = true; g.use.style.opacity = String(g.rest); } }
        else fadeOut(g, 1200, false);
      }
    }
    checkRef.current = check;

    function boot() {
      for (const t of timers) clearTimeout(t.id);
      timers.clear();
      svg!.querySelectorAll("g[data-glyph]").forEach((n) => n.remove());
      busy = 0; W = window.innerWidth; H = window.innerHeight;
      const phone = W < 640;
      const [lo, hi] = phone ? [5, 7] : [12, 18];
      const n = Math.round(Math.min(hi * density, Math.max(lo * density, (W * H / AREA_PER_GLYPH) * density)));
      minGap = Math.max(70, Math.sqrt((W * H) / Math.max(n, 1)) * .55);

      glyphs = Array.from({ length: n }, () => {
        const wrap = document.createElementNS(NS, "g") as SVGGElement;
        const use = document.createElementNS(NS, "use") as SVGUseElement;
        wrap.dataset.glyph = "";
        use.setAttribute("href", "#astralis-glyph");
        use.setAttribute("class", Math.random() < .25 ? "glyph glyph-accent" : "glyph");
        wrap.append(use); svg!.append(wrap);
        return { wrap, use, rest: rand(.06, .2), size: 0, cx: -999, cy: -999, placed: false, visible: false, leaving: false, gen: 0 };
      });

      for (const g of glyphs) {
        if (still) {
          if (place(g)) { g.visible = true; style(g, "none", g.rest, "none"); }
        } else if (startAtRest && Math.random() < .66 && place(g)) {
          g.visible = true;
          style(g, "none", g.rest, "none");
          later(() => { if (g.gen === 0) whenFree(() => (g.gen === 0 ? fadeOut(g, rand(3000, 5000)) : busy--)); }, rand(2000, 22000));
        } else {
          // Staggered first arrivals, so the field gathers slowly.
          later(() => { if (place(g)) whenFree(() => fadeIn(g)); else gone(g); }, rand(1500, startAtRest ? 20000 : 28000));
        }
      }
    }

    boot();
    const zoneTick = window.setInterval(() => { if (!document.hidden) check(); }, ZONE_CHECK_MS);
    let rt = 0;
    const onResize = () => { clearTimeout(rt); rt = window.setTimeout(boot, 400); };
    window.addEventListener("resize", onResize, { passive: true });

    return () => {
      alive = false;
      for (const t of timers) clearTimeout(t.id);
      clearInterval(zoneTick); clearTimeout(rt);
      checkRef.current = () => {};
      window.removeEventListener("resize", onResize);
      document.removeEventListener("visibilitychange", onVis);
      svg.querySelectorAll("g[data-glyph]").forEach((n) => n.remove());
    };
  }, [reduced, density, startAtRest]);

  return (
    <svg ref={svgRef} aria-hidden className="glyph-field pointer-events-none fixed inset-0 -z-10 h-full w-full">
      <defs>
        {/* The glyph, drawn once from the shared geometry exported by
            components/Butterflies.tsx — the same shape that perches in the
            app's empty states. Non-scaling strokes keep it at 1px from 10px
            to 22px. */}
        <symbol id="astralis-glyph" viewBox="0 0 24 24">
          <g id="astralis-glyph-half">
            {GLYPH_HALF.map((d) => (
              <path key={d} vectorEffect="non-scaling-stroke" d={d} />
            ))}
          </g>
          <use href="#astralis-glyph-half" transform={MIRROR} />
          <path vectorEffect="non-scaling-stroke" d={GLYPH_BODY} />
        </symbol>
      </defs>
    </svg>
  );
}
