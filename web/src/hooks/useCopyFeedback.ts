import { useCallback, useState } from "react";

/**
 * useCopyFeedback returns a boolean flag that flips true when markCopied is
 * called and back to false after delayMs. reset() flips it back early — used
 * by callers that need to clear the indicator when their context changes.
 * markCopied and reset have stable identities across renders.
 */
export function useCopyFeedback(delayMs: number = 1500): {
  copied: boolean;
  markCopied: () => void;
  reset: () => void;
} {
  const [copied, setCopied] = useState(false);
  const markCopied = useCallback(() => {
    setCopied(true);
    setTimeout(() => setCopied(false), delayMs);
  }, [delayMs]);
  const reset = useCallback(() => setCopied(false), []);
  return { copied, markCopied, reset };
}
