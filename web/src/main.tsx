import { QueryCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import ReactDOM from "react-dom/client";
import App from "@/App";
import { AuthGate } from "@/AuthGate";
import { ApiError } from "@/api/client";
import "@/index.css";

const client = new QueryClient({
  queryCache: new QueryCache({
    onError: (error, query) => {
      // A session that expires mid-use surfaces as a 401 on some query; flip
      // the auth gate so the login screen reappears instead of the app erroring
      // in place. Harmless when SSO is off (nothing 401s). Exclude the ["me"]
      // query itself so its own 401 (the signed-out state) can't loop.
      if (error instanceof ApiError && error.status === 401 && query.queryKey[0] !== "me") {
        client.invalidateQueries({ queryKey: ["me"] });
      }
    },
  }),
  defaultOptions: {
    queries: {
      staleTime: 5_000,
      refetchOnWindowFocus: false,
    },
  },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={client}>
      <AuthGate>
        <App />
      </AuthGate>
    </QueryClientProvider>
  </React.StrictMode>,
);
