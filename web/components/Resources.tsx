"use client";

import { useState } from "react";

import Butterfly from "@/components/Butterflies";
import Pacer from "@/components/Pacer";
import Reading from "@/components/Reading";
import RippleCard from "@/components/RippleCard";
import { HELPLINES, PRACTICES, RESEARCH, type Link, type Practice } from "@/lib/resources";

type Tab = "practices" | "reading" | "research" | "help";

const TABS: { id: Tab; label: string }[] = [
  { id: "practices", label: "Practices" },
  { id: "reading",   label: "Reading" },
  { id: "research",  label: "Research" },
  { id: "help",      label: "Real help" },
];

/**
 * Things to read and things to do.
 *
 * Every practice states its mechanism, because people abandon an instruction
 * and keep an explanation. Every link is real and checked — nothing generated,
 * nothing affiliate, nothing that claims to treat anything.
 */
export default function Resources() {
  const [tab, setTab] = useState<Tab>("practices");

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-3xl px-5 py-7 sm:px-8">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="label">Practices &amp; resources</p>
            <h1 className="mt-3 font-sans text-[1.75rem] font-semibold leading-tight tracking-[-.025em]">
              Things that help, and why they do
            </h1>
          </div>
          {/* Settled in the corner: these are here whenever, not now. */}
          <Butterfly pose="rest" className="mt-1 h-5 w-5 shrink-0 text-violet-2/55" />
        </div>

        <div role="tablist" aria-label="Resource categories"
             className="mt-6 flex flex-wrap gap-1.5 border-b border-line pb-3">
          {TABS.map((t) => (
            <button key={t.id} role="tab" aria-selected={tab === t.id}
                    onClick={() => setTab(t.id)}
                    className={`rounded-lg px-3 py-1.5 font-mono text-[.6875rem] uppercase
                                tracking-[.1em] transition-colors duration-150 ease-swift
                                ${tab === t.id
                                  ? "bg-violet/15 text-violet-2"
                                  : "text-ink-3 hover:bg-raised hover:text-ink-2"}`}>
              {t.label}
            </button>
          ))}
        </div>

        <div className="mt-7 animate-fade">
          {tab === "practices" && (
            <div className="grid gap-3 sm:grid-cols-2">
              {PRACTICES.map((p) => <PracticeCard key={p.name} p={p} />)}
            </div>
          )}
          {tab === "reading"  && <Reading />}
          {tab === "research" && <LinkList items={RESEARCH} />}
          {tab === "help" && (
            <>
              <p className="mb-5 max-w-measure text-[.875rem] leading-relaxed text-ink-2">
                Astralis is not a crisis service and never will be. These are people who
                are trained for exactly that, and they are free.
              </p>
              <LinkList items={HELPLINES} tone="amber" />
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function PracticeCard({ p }: { p: Practice }) {
  return (
    <RippleCard
      label={`Show steps for ${p.name}`}
      announce={`${p.name}. ${p.steps.map((s, i) => `Step ${i + 1}: ${s}`).join(" ")}`}
      front={
        <>
          <div className="flex items-start justify-between gap-3">
            <h2 className="font-sans text-[.9375rem] font-medium leading-snug">{p.name}</h2>
            <span className="tag shrink-0">{p.minutes}</span>
          </div>
          <p className="mt-2.5 text-[.8125rem] leading-[1.65] text-ink-2">{p.why}</p>
          <p className="mt-auto pt-3 font-mono text-[.625rem] uppercase tracking-[.12em] text-violet-2">
            tap to see the steps
          </p>
        </>
      }
      back={(open) => (
        <>
          <h3 className="-mt-4 pr-16 font-sans text-[.9375rem] font-medium leading-snug">{p.name}</h3>
          <ol className="mt-3 space-y-1.5 border-l border-line pl-3.5">
            {p.steps.map((s, i) => (
              <li key={i} className="flex gap-2.5 text-[.8125rem] leading-relaxed text-ink-2">
                <span className="font-mono text-[.625rem] tabular-nums text-ink-3">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <span>{s}</span>
              </li>
            ))}
          </ol>
          {p.pace && <Pacer pace={p.pace} active={open} />}
        </>
      )}
    />
  );
}

function LinkList({ items, tone }: { items: Link[]; tone?: "amber" }) {
  return (
    <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
      {items.map((l) => (
        <li key={l.label}>
          <a href={l.href}
             target={l.href.startsWith("http") ? "_blank" : undefined}
             rel="noreferrer noopener"
             className="group flex items-start gap-3 px-4 py-3.5 transition-colors hover:bg-raised">
            <span aria-hidden
                  className={`mt-[.6em] h-px w-3 shrink-0 ${tone === "amber" ? "bg-amber/70" : "bg-violet/60"}`} />
            <span className="min-w-0">
              <span className={`block font-sans text-[.875rem] font-medium
                                ${tone === "amber" ? "text-amber" : "text-ink"}`}>
                {l.label}
              </span>
              <span className="mt-0.5 block text-[.8125rem] leading-relaxed text-ink-2">
                {l.detail}
              </span>
            </span>
            <span aria-hidden
                  className="ml-auto mt-1 shrink-0 font-mono text-[.75rem] text-ink-3
                             transition-transform duration-150 ease-swift group-hover:translate-x-0.5">
              ›
            </span>
          </a>
        </li>
      ))}
    </ul>
  );
}
