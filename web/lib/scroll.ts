/**
 * scroll a pane to its end without touching anything around it.
 *
 * scrollIntoView walks every ancestor, including ones clipped with
 * overflow:hidden, so on a full thread it shoved the whole app shell upward.
 * setting scrollTop on the pane itself can only ever move that pane
 */
export function scrollPaneToEnd(pane: HTMLElement | null, smooth: boolean): void {
  pane?.scrollTo({ top: pane.scrollHeight, behavior: smooth ? "smooth" : "auto" });
}
