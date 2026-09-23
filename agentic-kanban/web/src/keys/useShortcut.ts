import { useEffect, useRef } from "react";
import { register } from "./dispatcher";

export function useShortcut(
  actionId: string,
  handler: () => void,
  options: { enabled?: boolean } = {},
): void {
  const enabled = options.enabled ?? true;
  const handlerRef = useRef(handler);
  handlerRef.current = handler;

  useEffect(() => {
    if (!enabled) return;
    return register(actionId, () => handlerRef.current());
  }, [actionId, enabled]);
}
