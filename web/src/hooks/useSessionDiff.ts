import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { queryKeys } from "@/api/keys";

// While the diff tab is open we poll fast for a near-live view; otherwise we
// poll slowly in the background purely to keep the react-query cache warm, so
// switching to the diff tab renders from cache instead of a cold fetch.
const ACTIVE_POLL_MS = 4000;
const BACKGROUND_POLL_MS = 10000;

// Shared diff query so the diff tab (DiffPanel) and the background warmer
// (SessionView) hit the same cache entry under `queryKeys.sessionDiff`. The two
// callers are mutually exclusive — SessionView only warms while the diff tab is
// closed, DiffPanel only mounts while it's open — so there's never a duplicate
// poll racing the cache.
export function useSessionDiff(
  sessionId: number,
  { enabled = true, active = true }: { enabled?: boolean; active?: boolean } = {},
) {
  return useQuery({
    queryKey: queryKeys.sessionDiff(sessionId),
    queryFn: () => api.getSessionDiff(sessionId),
    enabled,
    refetchInterval: active ? ACTIVE_POLL_MS : BACKGROUND_POLL_MS,
  });
}
