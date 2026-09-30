"use client";

import { useEffect, useMemo, useState } from "react";

import RippleCard from "@/components/RippleCard";
import { api, type LatestItem } from "@/lib/api";
import { timeAgo } from "@/lib/format";
import { ARTICLES, FACTS, type Article, type ReadingCategory } from "@/lib/resources";

type Filter = "all" | "latest" | ReadingCategory | "facts";

const FILTERS: { id: Filter; label: string }[] = [
  { id: "all",     label: "All" },
  { id: "latest",  label: "Latest (finance, tech, business)" },
  { id: "health",  label: "Health and mind" },
  { id: "science", label: "Science and nerdy stuff" },
  { id: "work",    label: "Work and life" },
  { id: "facts",   label: "Did you know?" },
];

const CURATED: { id: ReadingCategory; label: string }[] = [
  { id: "health",  label: "Health and mind" },
  { id: "science", label: "Science and nerdy stuff" },
  { id: "work",    label: "Work and life" },
];

const CATEGORY_LABEL: Record<LatestItem["category"] | ReadingCategory, string> = {
  finance:  "Finance",
  tech:     "Tech",
  business: "Business",
  health:   "Health and mind",
  science:  "Science",
  work:     "Work and life",
};

type Latest =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; items: LatestItem[] };

/**
 * Articles that open straight away: free, no paywall, no sign-up. Curated
 * pieces and facts are static; "Latest" comes from the gateway, which reads a
 * fixed list of public feeds. Nothing here records what anyone opens.
 */
export default function Reading() {
  const [filter, setFilter] = useState<Filter>("all");
  const [latest, setLatest] = useState<Latest>({ status: "idle" });

  useEffect(() => {
    if ((filter !== "all" && filter !== "latest") || latest.status !== "idle") return;
    setLatest({ status: "loading" });
    api<{ items: LatestItem[] }>("/reading/latest")
      .then(({ items }) => setLatest({ status: "ready", items }))
      .catch(() => setLatest({ status: "error" }));
  }, [filter, latest.status]);

  return (
    <div>
      <div role="group" aria-label="Filter reading" className="flex flex-wrap gap-1.5">
        {FILTERS.map((f) => (
          <button key={f.id} type="button" aria-pressed={filter === f.id}
                  onClick={() => setFilter(f.id)}
                  className={`rounded-full border px-3 py-1 font-sans text-[.75rem]
                              transition-colors duration-150 ease-swift
                              ${filter === f.id
                                ? "border-violet/50 bg-violet/15 text-violet-2"
                                : "border-line text-ink-3 hover:bg-raised hover:text-ink-2"}`}>
            {f.label}
          </button>
        ))}
      </div>

      <div key={filter} className="mt-6 animate-fade space-y-9">
        {filter === "all" && (
          <>
            <FeaturedFact />
            <Section title="Latest">
              <LatestList latest={latest} limit={4} fallback={false} />
            </Section>
            {CURATED.map((c) => (
              <Section key={c.id} title={c.label}>
                <ArticleGrid items={ARTICLES.filter((a) => a.category === c.id)} />
              </Section>
            ))}
          </>
        )}
        {filter === "latest" && <LatestList latest={latest} fallback />}
        {(filter === "health" || filter === "science" || filter === "work") && (
          <ArticleGrid items={ARTICLES.filter((a) => a.category === filter)} />
        )}
        {filter === "facts" && (
          <>
            <FeaturedFact />
            <FactList />
          </>
        )}
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h2 className="label mb-3">{title}</h2>
      {children}
    </section>
  );
}

function ArticleGrid({ items }: { items: Article[] }) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {items.map((a) => (
        <StoryCard key={a.url} source={a.source} category={CATEGORY_LABEL[a.category]}
                   meta={`${a.minutes} min read`} title={a.title} summary={a.summary} url={a.url} />
      ))}
    </div>
  );
}

function LatestList({ latest, limit, fallback }: {
  latest: Latest; limit?: number; fallback: boolean;
}) {
  if (latest.status === "idle" || latest.status === "loading") {
    return (
      <div className="grid gap-3 sm:grid-cols-2" role="status" aria-label="Loading the latest headlines">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="card space-y-3 p-4" aria-hidden>
            <div className="skel h-[.5rem] w-1/3" />
            <div className="skel h-[.625rem] w-11/12" />
            <div className="skel h-[.625rem] w-2/3" />
          </div>
        ))}
      </div>
    );
  }

  if (latest.status === "error") {
    return (
      <>
        <Notice>
          The latest headlines could not be loaded just now.
          {fallback ? " Here is the reading list instead." : " Try again a little later."}
        </Notice>
        {fallback && (
          <div className="mt-6 space-y-9">
            {CURATED.map((c) => (
              <Section key={c.id} title={c.label}>
                <ArticleGrid items={ARTICLES.filter((a) => a.category === c.id)} />
              </Section>
            ))}
          </div>
        )}
      </>
    );
  }

  if (latest.items.length === 0) {
    return <Notice>Nothing new has come through yet. Try again a little later.</Notice>;
  }

  const items = limit ? latest.items.slice(0, limit) : latest.items;
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {items.map((it) => (
        <StoryCard key={it.url} source={it.source} category={CATEGORY_LABEL[it.category]}
                   meta={timeAgo(it.publishedAt)} title={it.title} summary={it.summary} url={it.url} />
      ))}
    </div>
  );
}

function Notice({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded-lg border border-line bg-surface px-4 py-3 text-[.8125rem] leading-relaxed text-ink-2">
      {children}
    </p>
  );
}

/** One article. Text only: no images or favicons are loaded from anywhere. */
function StoryCard({ source, category, meta, title, summary, url }: {
  source: string; category: string; meta: string; title: string; summary: string; url: string;
}) {
  return (
    <RippleCard
      label={`Preview: ${title}`}
      announce={`${title}. ${summary}`}
      front={
        <>
          <div className="flex flex-wrap items-center gap-2">
            <span className="tag">{source}</span>
            <span className="label">{category}</span>
            <span className="ml-auto font-mono text-[.625rem] text-ink-3">{meta}</span>
          </div>
          <h3 className="mt-3 font-sans text-[.9375rem] font-medium leading-snug">{title}</h3>
          {summary && (
            <p className="mt-2 line-clamp-2 text-[.8125rem] leading-[1.65] text-ink-2">{summary}</p>
          )}
        </>
      }
      back={() => (
        <>
          <p className="-mt-4 pr-16 font-mono text-[.625rem] uppercase tracking-[.12em] text-ink-3">
            {source} · {meta}
          </p>
          <p className="mt-2 text-[.875rem] leading-[1.7] text-ink">
            {summary || "This source does not send a summary. The link opens the article itself."}
          </p>
          <a href={url} target="_blank" rel="noreferrer noopener"
             className="btn-primary mt-auto self-start !px-3 !py-1.5">
            Read at {source} <span aria-hidden className="font-mono">›</span>
          </a>
        </>
      )}
    />
  );
}

function dayOfYear(d: Date): number {
  return Math.floor((d.getTime() - new Date(d.getFullYear(), 0, 0).getTime()) / 86_400_000);
}

function FeaturedFact() {
  const fact = useMemo(() => FACTS[dayOfYear(new Date()) % FACTS.length]!, []);
  return (
    <figure className="card border-violet/30 p-5 sm:p-6">
      <p className="label text-violet-2">Did you know?</p>
      <blockquote className="mt-3 font-sans text-[1.0625rem] leading-[1.6] text-ink">
        {fact.text}
      </blockquote>
      <figcaption className="mt-3 font-mono text-[.6875rem] text-ink-3">
        Source:{" "}
        <a href={fact.url} target="_blank" rel="noreferrer noopener"
           className="text-violet-2 underline decoration-violet/40 underline-offset-2 hover:text-violet">
          {fact.source}
        </a>
      </figcaption>
    </figure>
  );
}

function FactList() {
  const today = useMemo(() => dayOfYear(new Date()) % FACTS.length, []);
  return (
    <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
      {FACTS.filter((_, i) => i !== today).map((f) => (
        <li key={f.text} className="px-4 py-3.5">
          <p className="text-[.875rem] leading-relaxed text-ink-2">{f.text}</p>
          <a href={f.url} target="_blank" rel="noreferrer noopener"
             className="mt-1 inline-block font-mono text-[.625rem] text-ink-3 underline
                        decoration-line underline-offset-2 hover:text-ink-2">
            {f.source}
          </a>
        </li>
      ))}
    </ul>
  );
}
