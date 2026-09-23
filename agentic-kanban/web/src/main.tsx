import React from "react";
import ReactDOM from "react-dom/client";
import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "@/App";
import { ApiError, formatApiError, postError } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { installRuntimeReporting } from "@/instrumentation/runtimeReport";
import { ToastProvider, useToast } from "@/toast";
import "@/index.css";

installRuntimeReporting();

// reportRuntimeError forwards unexpected errors to the backend. 4xx ApiErrors
// are user-visible business errors (board renamed in another tab, validation
// failures, …), not bugs — those toast but don't get reported.
function reportRuntimeError(err: unknown, source: string) {
  if (err instanceof ApiError && err.status < 500) return;
  void postError({
    message: formatApiError(err),
    stack: err instanceof Error ? err.stack : undefined,
    source,
  });
}

function Root() {
  const { push } = useToast();
  const [client] = React.useState(() => {
    const qc: QueryClient = new QueryClient({
      defaultOptions: {
        queries: {
          staleTime: 5_000,
          refetchOnWindowFocus: false,
          // Never retry 4xx — the response won't change (a 404 board stays
          // deleted, a 400 stays malformed). Retrying just amplified the
          // toast/report storm 4x for the board-deleted case below.
          retry: (count, err) =>
            !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 3,
        },
      },
      queryCache: new QueryCache({
        onError: (err, query) => {
          // A board's /state 404 means the board was deleted out from under
          // this client. There's no board-level SSE to keep the boards list
          // live, so a long-lived tab keeps a deleted board mounted. Since
          // 26713ba the multiplexed /api/events stream reliably delivers every
          // subscribed board's events (the old per-board EventSources were
          // starved by the browser's 6-connection cap and silently dropped
          // these), so each event for the dead board now re-invalidates and
          // refetches its 404ing /state on a loop. Refetch the boards list so
          // its Overview node unmounts and stops re-subscribing — and stay
          // quiet, since this is expected churn, not a bug to surface.
          if (
            err instanceof ApiError &&
            err.status === 404 &&
            Array.isArray(query.queryKey) &&
            query.queryKey[0] === "board"
          ) {
            void qc.invalidateQueries({ queryKey: queryKeys.boards });
            return;
          }
          push("error", formatApiError(err));
          reportRuntimeError(err, "react-query");
        },
      }),
      mutationCache: new MutationCache({
        onError: (err) => {
          push("error", formatApiError(err));
          reportRuntimeError(err, "react-query");
        },
      }),
    });
    return qc;
  });
  return (
    <QueryClientProvider client={client}>
      <ErrorBoundary>
        <App />
      </ErrorBoundary>
    </QueryClientProvider>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ToastProvider>
      <Root />
    </ToastProvider>
  </React.StrictMode>,
);
