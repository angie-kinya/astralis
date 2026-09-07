/** Group sessions the way a person thinks about them, not by raw timestamp. */
export function dayBucket(iso: string): string {
  const d = new Date(iso);
  const today = new Date();
  const startOf = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const days = Math.round((startOf(today) - startOf(d)) / 86_400_000);

  if (days <= 0) return "Today";
  if (days === 1) return "Yesterday";
  if (days < 7) return "This week";
  if (days < 30) return "This month";
  return d.toLocaleDateString("en-GB", { month: "long", year: "numeric" });
}

export function timeOfDay(iso: string): string {
  return new Date(iso).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });
}

/**
 * The model writes plain prose with occasional **bold**. Rather than pull in a
 * markdown renderer (and its XSS surface) for two features, split on blank
 * lines and handle bold with a single pass. Anything we do not understand is
 * rendered as text, never as HTML.
 */
export function paragraphs(text: string): string[] {
  return text.trim().split(/\n{2,}/).filter(Boolean);
}
