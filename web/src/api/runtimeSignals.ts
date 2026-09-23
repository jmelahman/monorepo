// Framework-agnostic runtime signals surfaced by the developer toolbar.
//
// Deliberately dependency-free (no imports from client.ts or store.ts) so
// api/client.ts can import it without a circular module-initialization order:
// store.ts already runtime-imports client.ts, so client.ts must not reach back
// into store.ts for the ScalarStore class. This tiny self-contained Signal
// fills that gap.

type Listener = () => void;

class Signal<T> {
  private value: T;
  private listeners = new Set<Listener>();

  constructor(initial: T) {
    this.value = initial;
  }

  get = (): T => this.value;

  set = (next: T): void => {
    if (Object.is(next, this.value)) return;
    this.value = next;
    for (const l of this.listeners) l();
  };

  subscribe = (cb: Listener): (() => void) => {
    this.listeners.add(cb);
    return () => {
      this.listeners.delete(cb);
    };
  };
}

// Number of HTTP requests currently in flight through the api client's
// request() wrapper. Long-lived SSE/WebSocket streams are tracked separately
// (see sseStatus) and intentionally excluded.
export const inflightRequests = new Signal(0);

export function trackRequestStart(): void {
  inflightRequests.set(inflightRequests.get() + 1);
}

export function trackRequestEnd(): void {
  inflightRequests.set(Math.max(0, inflightRequests.get() - 1));
}

export type SseStatus = "open" | "error" | "closed";

// Status of the single multiplexed board EventSource. One global value is
// representative because every board subscription shares one underlying stream
// (see BoardEventManager in client.ts).
export const sseStatus = new Signal<SseStatus>("closed");
