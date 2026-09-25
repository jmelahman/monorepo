import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/api/client";
import { Button, ErrorText, inputClass } from "@/components/ui";

export default function Login() {
  const qc = useQueryClient();
  const [secret, setSecret] = useState("");
  const login = useMutation({
    mutationFn: api.login,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["health"] }),
  });
  return (
    <div className="flex min-h-dvh items-center justify-center bg-bg p-6 text-fg">
      <form
        className="w-full max-w-sm space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (secret) login.mutate(secret);
        }}
      >
        <h1 className="text-2xl font-semibold">Welcome back</h1>
        <p className="text-sm text-fg-muted">Enter the secret this server was started with.</p>
        <input
          type="password"
          autoComplete="current-password"
          aria-label="Secret"
          value={secret}
          onChange={(e) => setSecret(e.target.value)}
          className={inputClass}
          // biome-ignore lint/a11y/noAutofocus: the only field on the page
          autoFocus
        />
        <ErrorText error={login.error} />
        <Button type="submit" disabled={!secret || login.isPending} className="w-full">
          {login.isPending ? "Checking…" : "Open"}
        </Button>
      </form>
    </div>
  );
}
