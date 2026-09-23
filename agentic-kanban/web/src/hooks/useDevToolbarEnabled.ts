import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { queryKeys } from "@/api/keys";

// useDevToolbarEnabled reads the opt-in `dev_toolbar.enabled` config flag from
// the merged (effective) config. Returns false until the flag is explicitly
// set true in `.kanban.toml` or the user config, so the developer toolbar and
// its header button stay hidden for everyone who hasn't opted in.
//
// Config rarely changes at runtime, so this is cached aggressively; flipping
// the flag takes effect on the next page load.
export function useDevToolbarEnabled(): boolean {
  const { data } = useQuery({
    queryKey: queryKeys.config("effective"),
    queryFn: () => api.getConfig("effective"),
    staleTime: 5 * 60 * 1000,
    gcTime: Number.POSITIVE_INFINITY,
  });
  const entry = data?.entries.find((e) => e.key === "dev_toolbar.enabled");
  return entry?.value === true;
}
