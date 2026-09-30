import Link from "next/link";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import ThemeToggle from "@/components/ThemeToggle";
import Wordmark from "@/components/Wordmark";

/**
 * The landing page.
 *
 * Audience: engineers, researchers, and anyone technical enough to want to
 * know how a thing works before they trust it with something personal. So it
 * states the mechanism plainly and briefly, then gets out of the way. No
 * testimonials, no feature grid, no scroll-jacking — the people this is for
 * find all of that suspicious, and they are right to.
 */
const MECHANISM = [
  {
    k: "retrieval",
    t: "It reads the literature",
    d: "A scheduled job pulls peer-reviewed work on occupational stress from OpenAlex, chunks it, embeds it into pgvector. The library grows weekly. No model is retrained.",
  },
  {
    k: "memory",
    t: "It remembers your thread",
    d: "A short factual summary per session, retrieved as context next time. Stored as rows in Postgres, auditable and deletable in one click.",
  },
  {
    k: "feedback",
    t: "It learns what helped",
    d: "Mark a reply useful and the passages behind it are reweighted for retrieval. That reweighting is the whole of its self-improvement.",
  },
  {
    k: "safety",
    t: "It knows its limits",
    d: "A rules-plus-classifier filter runs before the model. Flagged messages never reach it, never enter retrieval, never enter memory. They get a fixed card written by a person.",
  },
];

export default async function Landing() {
  if (cookies().get("astralis_session")) redirect("/chat");

  return (
    <div className="relative min-h-dvh">
      <header className="mx-auto flex max-w-4xl items-center justify-between px-6 py-6">
        <Wordmark />
        <ThemeToggle />
      </header>

      <main className="mx-auto max-w-4xl px-6 pb-24">
        <section className="pt-12 sm:pt-20">
          <p className="label">Closed beta · Independent research project</p>

          <h1 className="mt-5 max-w-[16ch] font-sans text-[2.5rem] font-semibold
                         leading-[1.06] tracking-[-.03em] sm:text-[3.25rem]">
            A quieter place to think about work.
          </h1>

          <p className="mt-6 max-w-measure text-[1rem] leading-[1.72] text-ink-2">
            Astralis is a reflection companion for burnout and work stress. When evidence
            would help, it draws on published research. It keeps track of your thread
            between conversations and connects you with other people using it, under
            whatever name you choose.
          </p>

          <div className="mt-9 flex flex-wrap items-center gap-3">
            <Link href="/signin" className="btn-primary">
              Begin with your email
              <span aria-hidden className="font-mono">›</span>
            </Link>
            <span className="font-mono text-[.6875rem] leading-relaxed text-ink-3">
              No account elsewhere needed. Others here see
              <br className="hidden sm:block" />
              only the name you choose, never your email.
            </span>
          </div>
        </section>

        <div className="hair my-14 sm:my-20" />

        <section>
          <p className="label">How it works</p>
          <div className="mt-7 grid gap-x-10 gap-y-9 sm:grid-cols-2">
            {MECHANISM.map((m) => (
              <div key={m.k} className="border-l border-line pl-5">
                <span className="font-mono text-[.625rem] uppercase tracking-[.14em] text-violet-2">
                  {m.k}
                </span>
                <h2 className="mt-2 text-[1.0625rem] font-medium leading-snug tracking-[-.015em]">
                  {m.t}
                </h2>
                <p className="mt-2 text-[.875rem] leading-[1.68] text-ink-2">{m.d}</p>
              </div>
            ))}
          </div>
        </section>

        <div className="hair my-14 sm:my-20" />

        <section className="grid gap-8 sm:grid-cols-[1fr_auto]">
          <div>
            <p className="label">What it is not</p>
            <ul className="mt-5 space-y-3 text-[.875rem] leading-relaxed text-ink-2">
              {[
                "Not therapy, not diagnosis, not a clinical tool of any kind.",
                "Not a crisis service. Crisis language is intercepted before the model sees it.",
                "Not anonymous to the operator: messages are stored, and processed by Anthropic's API.",
                "Not a product. There is no company, no pricing page, and no plan to build one.",
              ].map((t) => (
                <li key={t} className="flex gap-3">
                  <span aria-hidden className="mt-[.6em] h-px w-3 shrink-0 bg-line-2" />
                  {t}
                </li>
              ))}
            </ul>
          </div>

          <aside className="card self-start p-5 font-mono text-[.6875rem] leading-[1.9] text-ink-3">
            <p className="text-ink-2">stack</p>
            <p>go · gin · asynq</p>
            <p>postgres 16 · pgvector</p>
            <p>python · scikit-learn</p>
            <p>next.js · tailwind</p>
          </aside>
        </section>
      </main>

      <footer className="mx-auto max-w-4xl px-6 pb-10">
        <div className="hair mb-6" />
        <p className="font-mono text-[.6875rem] leading-relaxed text-ink-3">
          Experimental project. Not a substitute for professional care. In immediate
          danger, call 999 or 112. Kenya: Befrienders +254&nbsp;722&nbsp;178&nbsp;177.
        </p>
      </footer>
    </div>
  );
}
