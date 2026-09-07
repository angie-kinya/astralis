import { redirect } from "next/navigation";

import ThemeToggle from "@/components/ThemeToggle";
import Wordmark from "@/components/Wordmark";
import { auth, signIn } from "@/auth";

// Deliberately honest about the mechanism. People are being asked to talk
// about something difficult with a machine; the least it can do is explain
// exactly what it is before they start.
const MECHANISM = [
  {
    n: "01",
    title: "It reads the literature",
    body: "A scheduled job pulls peer-reviewed work on occupational stress from OpenAlex, splits it into passages, and indexes it. The library grows every week without anyone retraining anything.",
  },
  {
    n: "02",
    title: "It remembers your thread",
    body: "After each conversation a short factual summary is kept — your situation, what you have tried, what landed. Next time, that is the context it starts from.",
  },
  {
    n: "03",
    title: "It learns what actually helped",
    body: "When you mark a reply useful, the passages behind it are weighted up for everyone. That reweighting is the whole of its self-improvement. No model is trained on your words.",
  },
];

export default async function Landing() {
  const session = await auth();
  if (session?.user) redirect("/chat");

  return (
    <div className="relative min-h-dvh">
      <header className="mx-auto flex max-w-5xl items-center justify-between px-6 py-6">
        <Wordmark />
        <ThemeToggle />
      </header>

      <main className="mx-auto max-w-5xl px-6 pb-32">
        <section className="grid gap-12 pt-10 md:grid-cols-[1.15fr_0.85fr] md:pt-20">
          <div>
            <p className="eyebrow">Closed beta · Personal research project</p>

            <h1 className="mt-5 max-w-[15ch] font-serif text-[2.75rem] font-normal
                           leading-[1.08] tracking-[-0.02em] text-ink sm:text-[3.5rem]">
              A quieter place to think about work.
            </h1>

            <p className="mt-6 max-w-measure text-[1.0625rem] leading-[1.68] text-ink-2">
              Astralis is a reflection companion for burnout and occupational stress.
              It reads published research so it can ground what it says in something
              other than its own confidence, and it keeps track of your thread across
              conversations so you are not starting over every time.
            </p>

            <form
              className="mt-9"
              action={async () => {
                "use server";
                await signIn("github", { redirectTo: "/chat" });
              }}
            >
              <button className="btn-primary" type="submit">
                <svg viewBox="0 0 16 16" className="h-4 w-4" fill="currentColor" aria-hidden>
                  <path d="M8 0C3.58 0 0 3.58 0 8a8 8 0 0 0 5.47 7.59c.4.07.55-.17.55-.38l-.01-1.49c-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.4 7.4 0 0 1 4 0c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48l-.01 2.19c0 .21.15.46.55.38A8 8 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
                </svg>
                Continue with GitHub
              </button>
            </form>

            <p className="mt-4 max-w-measure font-mono text-[0.6875rem] leading-relaxed text-ink-3">
              You will be shown exactly what is stored, and can delete all of it at any time.
            </p>
          </div>

          {/* Not decoration: a plain statement of what this is and is not, at the
              same visual weight as the pitch. */}
          <aside className="card self-start p-6 md:mt-14">
            <p className="eyebrow">What this is not</p>
            <ul className="mt-4 space-y-3 text-[0.875rem] leading-relaxed text-ink-2">
              <li className="flex gap-3">
                <span aria-hidden className="mt-[0.55em] h-px w-3 shrink-0 bg-line-strong" />
                Not therapy, not diagnosis, not a clinical tool of any kind.
              </li>
              <li className="flex gap-3">
                <span aria-hidden className="mt-[0.55em] h-px w-3 shrink-0 bg-line-strong" />
                Not a crisis service. Messages suggesting crisis are intercepted
                before the model sees them and answered with real helplines.
              </li>
              <li className="flex gap-3">
                <span aria-hidden className="mt-[0.55em] h-px w-3 shrink-0 bg-line-strong" />
                Not anonymous. Your messages are stored, and processed by
                Anthropic&rsquo;s API to generate replies.
              </li>
            </ul>
          </aside>
        </section>

        <div className="hairline my-20" />

        <section>
          <p className="eyebrow">How it works</p>
          <div className="mt-8 grid gap-x-10 gap-y-10 sm:grid-cols-3">
            {MECHANISM.map((m) => (
              <div key={m.n}>
                <span className="font-mono text-[0.6875rem] text-accent">{m.n}</span>
                <h3 className="mt-3 font-serif text-[1.1875rem] leading-snug text-ink">{m.title}</h3>
                <p className="mt-2.5 text-[0.875rem] leading-[1.65] text-ink-2">{m.body}</p>
              </div>
            ))}
          </div>
        </section>
      </main>

      <footer className="mx-auto max-w-5xl px-6 pb-10">
        <div className="hairline mb-6" />
        <p className="font-mono text-[0.6875rem] leading-relaxed text-ink-3">
          Astralis is an experimental student project and is not a substitute for
          professional care. If you are in immediate danger, call 999 or 112. In Kenya,
          Befrienders Kenya: +254 722 178 177.
        </p>
      </footer>
    </div>
  );
}
