import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router";
import { api, setUnauthorizedHandler } from "@/api/client";
import Layout from "@/components/Layout";
import Board from "@/pages/Board";
import Login from "@/pages/Login";
import Retro from "@/pages/Retro";
import Roadmap from "@/pages/Roadmap";
import Settings from "@/pages/Settings";
import Thoughts from "@/pages/Thoughts";
import Today from "@/pages/Today";

export default function App() {
  const qc = useQueryClient();
  const health = useQuery({ queryKey: ["health"], queryFn: api.health, staleTime: 30_000 });

  // Any 401 means the cookie expired or the secret changed: re-check health,
  // which flips the app back to the login screen.
  useEffect(() => {
    setUnauthorizedHandler(() => qc.invalidateQueries({ queryKey: ["health"] }));
  }, [qc]);

  if (health.isPending) {
    return <div className="min-h-dvh bg-bg" />;
  }
  if (health.isError) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-bg p-6 text-center text-fg">
        <p>
          Can't reach the AgileCBT server.{" "}
          <button type="button" className="underline" onClick={() => health.refetch()}>
            Try again
          </button>
        </p>
      </div>
    );
  }
  if (health.data.auth_required && !health.data.authenticated) {
    return <Login />;
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Today health={health.data} />} />
          <Route path="board" element={<Board />} />
          <Route path="roadmap" element={<Roadmap />} />
          <Route path="thoughts" element={<Thoughts />} />
          <Route path="retro" element={<Retro health={health.data} />} />
          <Route path="settings" element={<Settings health={health.data} />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
