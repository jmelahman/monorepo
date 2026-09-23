import { type RefObject, useEffect } from "react";

type Options = {
  enabled?: boolean;
  // CSS selectors for elements that should NOT count as "outside" — clicks
  // on these are ignored. Useful for keeping the host open when the user
  // interacts with floating UI rendered as siblings (toasts, popovers).
  exemptSelectors?: string[];
};

export function useClickOutside(
  ref: RefObject<HTMLElement | null>,
  onOutside: (e: MouseEvent) => void,
  { enabled = true, exemptSelectors = [] }: Options = {},
): void {
  // We key on the joined selector string rather than the array identity so
  // callers don't need to memoize a stable array reference. Callers needing
  // dynamic selectors can rebuild the array each render — the join is what
  // determines re-subscription.
  // biome-ignore lint/correctness/useExhaustiveDependencies: see above.
  useEffect(() => {
    if (!enabled) return;
    const handler = (e: MouseEvent) => {
      const target = e.target as Element | null;
      if (!target) return;
      if (ref.current?.contains(target as Node)) return;
      for (const sel of exemptSelectors) {
        if (target.closest(sel)) return;
      }
      onOutside(e);
    };
    window.addEventListener("mousedown", handler);
    return () => window.removeEventListener("mousedown", handler);
  }, [enabled, ref, onOutside, exemptSelectors.join("|")]);
}
