import React from "react";
import { postError } from "@/api/client";

type Props = { children: React.ReactNode };
type State = { error: Error | null };

export class ErrorBoundary extends React.Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    void postError({
      message: error.message || "Unknown error",
      stack: error.stack ?? info.componentStack ?? "",
      source: "boundary",
    });
  }

  reset = () => {
    this.setState({ error: null });
  };

  render() {
    if (this.state.error) {
      return (
        <ErrorFallback
          error={this.state.error}
          onReload={() => window.location.reload()}
          onReset={this.reset}
        />
      );
    }
    return this.props.children;
  }
}

function ErrorFallback({
  error,
  onReload,
  onReset,
}: {
  error: Error;
  onReload: () => void;
  onReset: () => void;
}) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-bg p-8">
      <div className="max-w-lg space-y-4 text-center">
        <h1 className="text-2xl font-semibold text-fg">Something went wrong</h1>
        <p className="text-sm text-muted">
          The kanban UI hit an unexpected error. The error has been reported.
        </p>
        <pre className="rounded-md bg-surface p-3 text-left text-xs text-muted overflow-auto">
          {error.message}
        </pre>
        <div className="flex justify-center gap-2">
          <button
            type="button"
            onClick={onReset}
            className="rounded-md border border-border px-3 py-1.5 text-sm text-fg hover:bg-surface"
          >
            Try again
          </button>
          <button
            type="button"
            onClick={onReload}
            className="rounded-md bg-accent px-3 py-1.5 text-sm text-bg hover:opacity-90"
          >
            Reload
          </button>
        </div>
      </div>
    </div>
  );
}
