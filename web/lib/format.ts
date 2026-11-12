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

/** "30 September 2026" */
export function longDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric" });
}

/** "30 September 2026, 14:02" */
export function dateTime(iso: string): string {
  return `${longDate(iso)}, ${timeOfDay(iso)}`;
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

/** Coarse relative time. Exact minutes imply a precision presence does not have. */
export function relative(iso: string): string {
  const mins = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60_000));
  if (mins < 2) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.round(hrs / 24);
  // past a week the date itself is more useful than a vaguer phrase
  return days < 7 ? `${days}d ago` : dayLabel(iso);
}

/** "Today", "Yesterday", "Monday", "3 November", "3 November 2025" */
export function dayLabel(iso: string): string {
  const d = new Date(iso), now = new Date();
  const start = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const days = Math.round((start(now) - start(d)) / 86_400_000);
  if (days <= 0) return "Today";
  if (days === 1) return "Yesterday";
  if (days < 7) return d.toLocaleDateString("en-GB", { weekday: "long" });
  return d.toLocaleDateString("en-GB", d.getFullYear() === now.getFullYear()
    ? { day: "numeric", month: "long" }
    : { day: "numeric", month: "long", year: "numeric" });
}

/** a local calendar day, for deciding where a day separator goes */
export const dayKey = (iso: string) => new Date(iso).toDateString();

/** Relative time in words, for headlines: "just now", "3 hours ago", "2 days ago". */
export function timeAgo(iso: string): string {
  const mins = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60_000));
  if (mins < 2) return "just now";
  if (mins < 60) return `${mins} minutes ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return hrs === 1 ? "1 hour ago" : `${hrs} hours ago`;
  const days = Math.round(hrs / 24);
  if (days < 7) return days === 1 ? "yesterday" : `${days} days ago`;
  return new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "long" });
}
