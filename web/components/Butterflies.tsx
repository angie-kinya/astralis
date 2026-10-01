"use client";

import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";

import { useReducedMotion } from "@/lib/motion";

/**
 * A few butterflies crossing the background, on one fixed canvas behind
 * everything. Hand-rolled rather than particles.js (unmaintained) or
 * tsParticles (a dependency for six shapes).
 *
 * Care rules, in order:
 *   - Text wins. Any element marked data-butterfly-clear (the chat's reading
 *     column and composer, anything translucent over text) is a keep-out
 *     zone: a butterfly fades to nothing before its wings reach it.
 *   - Cheap. ~30fps cap, a handful of paths per frame, paused while the tab is
 *     hidden, devicePixelRatio capped at 2. Layout is read twice a second, never
 *     inside the draw.
 *   - Reduced motion: nothing is drawn at all.
 */

type RGB = [number, number, number];
type Fly = {
  p0: [number, number]; c: [number, number]; p1: [number, number];
  t: number; dur: number;      // progress 0..1 along the curve, seconds to cross
  size: number; alpha: number; // body length in px; base opacity
  phase: number; flapHz: number; glide: number;
  wobble: number; wobbleHz: number;
  mix: number;                 // how much cyan the hind wings carry
  angle: number; x: number; y: number;
};
type Zone = { l: number; t: number; r: number; b: number };

const rand = (a: number, b: number) => a + Math.random() * (b - a);
const FPS = 30;

function countFor(w: number) { return w < 640 ? 3 : w < 1024 ? 4 : w < 1600 ? 5 : 6; }

/** A slow curved crossing: left to right, or corner to corner, starting off-screen. */
function spawn(w: number, h: number, fresh: boolean): Fly {
  const m = 70;
  const routes: Array<() => [[number, number], [number, number]]> = [
    () => [[-m, rand(.1, .85) * h], [w + m, rand(.1, .85) * h]],          // left → right
    () => [[w + m, rand(.1, .85) * h], [-m, rand(.1, .85) * h]],          // right → left
    () => [[rand(-m, .25 * w), -m], [w + m, rand(.6, 1) * h + m]],         // top-left → bottom-right
    () => [[rand(-m, .25 * w), h + m], [w + m, rand(-m, .4 * h)]],         // bottom-left → top-right
    () => [[rand(.75 * w, w + m), -m], [-m, rand(.6, 1) * h + m]],         // top-right → bottom-left
  ];
  const [p0, p1] = routes[Math.floor(Math.random() * routes.length)]!();
  const dx = p1[0] - p0[0], dy = p1[1] - p0[1], len = Math.hypot(dx, dy);
  const bend = rand(-.3, .3) * len;
  const c: [number, number] = [(p0[0] + p1[0]) / 2 - (dy / len) * bend, (p0[1] + p1[1]) / 2 + (dx / len) * bend];
  const small = w < 640;
  return {
    p0, c, p1,
    t: fresh ? rand(0, .85) : 0,
    dur: (len * 1.1) / rand(16, 34),
    size: rand(small ? 11 : 13, small ? 18 : 24),
    alpha: rand(.15, .23),
    phase: rand(0, Math.PI * 2), flapHz: rand(1.3, 2.4), glide: rand(0, Math.PI * 2),
    wobble: rand(8, 26), wobbleHz: rand(.08, .2),
    mix: rand(.12, .45),
    angle: Math.atan2(dy, dx), x: p0[0], y: p0[1],
  };
}

function readColours(): { violet: RGB; violet2: RGB; cyan: RGB } {
  const cs = getComputedStyle(document.documentElement);
  const get = (n: string) => cs.getPropertyValue(n).trim().split(/\s+/).map(Number) as RGB;
  return { violet: get("--violet"), violet2: get("--violet-2"), cyan: get("--cyan") };
}

const mixRGB = (a: RGB, b: RGB, k: number) => a.map((v, i) => Math.round(v + (b[i]! - v) * k)) as RGB;
const rgba = (c: RGB, a: number) => `rgba(${c[0]},${c[1]},${c[2]},${a})`;

/** One wing pair on one side (s = -1 or 1); k is how open the wings are. */
function wings(ctx: CanvasRenderingContext2D, s: number, k: number, fore: string, hind: string) {
  ctx.fillStyle = hind;
  ctx.beginPath();
  ctx.moveTo(-.02, 0);
  ctx.bezierCurveTo(-.05, .55 * k * s, -.55, .78 * k * s, -.56, .46 * k * s);
  ctx.bezierCurveTo(-.56, .2 * k * s, -.25, .05 * k * s, -.02, 0);
  ctx.fill();
  ctx.fillStyle = fore;
  ctx.beginPath();
  ctx.moveTo(.05, 0);
  ctx.bezierCurveTo(.36, .25 * k * s, .58, .95 * k * s, .16, 1.02 * k * s);
  ctx.bezierCurveTo(-.1, 1.02 * k * s, -.16, .42 * k * s, 0, 0);
  ctx.fill();
}

export default function Butterflies() {
  const reduced = useReducedMotion();
  const canvas = useRef<HTMLCanvasElement>(null);
  // The landing page is an arrival: the same butterflies, a little brighter
  // and larger. Read per frame, so navigating never restarts the flight.
  const hero = useRef(false);
  hero.current = usePathname() === "/";

  useEffect(() => {
    const el = canvas.current;
    if (!el || reduced || matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const ctx = el.getContext("2d");
    if (!ctx) return;

    let w = 0, h = 0, dpr = 1, raf = 0, last = 0, acc = 0, clock = 0, nextZones = 0;
    let flies: Fly[] = [];
    let zones: Zone[] = [];
    let colours = readColours();

    function resize() {
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      w = window.innerWidth; h = window.innerHeight;
      el!.width = Math.round(w * dpr); el!.height = Math.round(h * dpr);
      const n = countFor(w);
      flies = flies.slice(0, n);
      while (flies.length < n) flies.push(spawn(w, h, true));
    }

    // Keep-out zones. Elements sharing a data-butterfly-clear value are
    // unioned into one box (the chat's column and composer are one vertical
    // band); an element with no value is a zone of its own.
    function readZones() {
      const boxes = new Map<string, Zone>();
      document.querySelectorAll<HTMLElement>("[data-butterfly-clear]").forEach((e, i) => {
        const q = e.getBoundingClientRect();
        if (!q.width || !q.height || q.bottom < 0 || q.top > h) return;
        const key = e.dataset.butterflyClear || `#${i}`, z = boxes.get(key);
        boxes.set(key, z
          ? { l: Math.min(z.l, q.left), t: Math.min(z.t, q.top), r: Math.max(z.r, q.right), b: Math.max(z.b, q.bottom) }
          : { l: q.left, t: q.top, r: q.right, b: q.bottom });
      });
      zones = [...boxes.values()];
    }

    function zoneFade(x: number, y: number, reach: number) {
      let f = 1;
      for (const z of zones) {
        const dx = Math.max(z.l - x, 0, x - z.r), dy = Math.max(z.t - y, 0, y - z.b);
        f = Math.min(f, Math.max(0, Math.min(1, (Math.hypot(dx, dy) - reach) / 70)));
      }
      return f;
    }

    function frame(now: number) {
      raf = requestAnimationFrame(frame);
      const dt = Math.min((now - (last || now)) / 1000, .1);
      last = now; acc += dt;
      if (acc < 1 / FPS) return;
      const step = acc; acc = 0; clock += step;
      if (now >= nextZones) { readZones(); nextZones = now + 450; }

      const isHero = hero.current;
      const boost = isHero ? 1.4 : 1, grow = isHero ? 1.25 : 1;
      ctx!.setTransform(1, 0, 0, 1, 0, 0);
      ctx!.clearRect(0, 0, el!.width, el!.height);

      flies.forEach((f, i) => {
        f.t += step / f.dur;
        if (f.t >= 1) { flies[i] = spawn(w, h, false); return; }
        const u = 1 - f.t;
        let x = u * u * f.p0[0] + 2 * u * f.t * f.c[0] + f.t * f.t * f.p1[0];
        let y = u * u * f.p0[1] + 2 * u * f.t * f.c[1] + f.t * f.t * f.p1[1];
        // A lazy side-to-side drift on top of the curve, so no path is a clean arc.
        const tx = 2 * u * (f.c[0] - f.p0[0]) + 2 * f.t * (f.p1[0] - f.c[0]);
        const ty = 2 * u * (f.c[1] - f.p0[1]) + 2 * f.t * (f.p1[1] - f.c[1]);
        const tl = Math.hypot(tx, ty) || 1, wob = Math.sin(clock * f.wobbleHz * Math.PI * 2 + f.phase) * f.wobble;
        x += (-ty / tl) * wob; y += (tx / tl) * wob;

        const heading = Math.atan2(y - f.y, x - f.x);
        if (Number.isFinite(heading) && (x !== f.x || y !== f.y)) {
          let d = heading - f.angle;
          d = Math.atan2(Math.sin(d), Math.cos(d));
          f.angle += d * Math.min(1, step * 3);
        }
        f.x = x; f.y = y;

        const size = f.size * grow;
        const a = f.alpha * boost * zoneFade(x, y, size);
        if (a < .01) return;

        // Flap in bursts with glides between, so each one has its own rhythm.
        f.phase += step * f.flapHz * Math.PI * 2 * (.55 + .45 * Math.sin(clock * .35 + f.glide));
        const k = .28 + .72 * Math.abs(Math.cos(f.phase));

        const fore = rgba(colours.violet2, a);
        const hind = rgba(mixRGB(colours.violet, colours.cyan, f.mix), a * .9);
        ctx!.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx!.translate(x, y);
        ctx!.rotate(f.angle);
        ctx!.scale(size, size);
        ctx!.shadowColor = rgba(colours.violet2, Math.min(1, a * 2.2));
        ctx!.shadowBlur = size * .7 * dpr;
        wings(ctx!, -1, k, fore, hind);
        wings(ctx!, 1, k, fore, hind);
        ctx!.shadowBlur = 0;
        ctx!.fillStyle = rgba(colours.violet2, Math.min(1, a * 1.6));
        ctx!.beginPath();
        ctx!.ellipse(-.02, 0, .38, .045, 0, 0, Math.PI * 2);
        ctx!.fill();
      });
    }

    function start() { if (!raf) { last = 0; raf = requestAnimationFrame(frame); } }
    function stop() { cancelAnimationFrame(raf); raf = 0; }
    const onVis = () => (document.hidden ? stop() : start());

    // The theme toggle flips data-theme on <html>; re-read the tokens then.
    const themeObs = new MutationObserver(() => { colours = readColours(); });
    themeObs.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

    resize();
    window.addEventListener("resize", resize, { passive: true });
    document.addEventListener("visibilitychange", onVis);
    if (!document.hidden) start();

    return () => {
      stop();
      themeObs.disconnect();
      window.removeEventListener("resize", resize);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [reduced]);

  if (reduced) return null;
  return <canvas ref={canvas} aria-hidden className="pointer-events-none fixed inset-0 -z-10 h-full w-full" />;
}
