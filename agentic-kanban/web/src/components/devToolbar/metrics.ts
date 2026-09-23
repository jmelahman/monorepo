import { useQueryClient } from "@tanstack/react-query";
import type { QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import { inflightRequests, sseStatus, type SseStatus } from "@/api/runtimeSignals";

// Live metric sources for the developer toolbar. Each sampler only runs while
// `active` is true, and the widget itself only mounts while open — so when the
// toolbar is closed nothing here costs anything (no rAF loop, no interval).

export type FrameStats = { fps: number; worstMs: number };

// useFrameStats drives a requestAnimationFrame loop while active, counting
// frames over a rolling 1s window for FPS and tracking the worst inter-frame
// gap as a jank signal. The first frame after (re)start is skipped so the time
// spent mounting the effect doesn't inflate the worst-frame number.
export function useFrameStats(active: boolean): FrameStats {
  const [stats, setStats] = useState<FrameStats>({ fps: 0, worstMs: 0 });
  useEffect(() => {
    if (!active) {
      setStats({ fps: 0, worstMs: 0 });
      return;
    }
    let raf = 0;
    let primed = false;
    let last = performance.now();
    let windowStart = last;
    let frames = 0;
    let worst = 0;
    const tick = (now: number) => {
      const delta = now - last;
      last = now;
      if (primed) {
        frames += 1;
        if (delta > worst) worst = delta;
      }
      primed = true;
      if (now - windowStart >= 1000) {
        setStats({
          fps: Math.round((frames * 1000) / (now - windowStart)),
          worstMs: Math.round(worst),
        });
        frames = 0;
        worst = 0;
        windowStart = now;
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [active]);
  return stats;
}

export type RuntimeSamples = {
  heapUsed: number | null;
  heapTotal: number | null;
  heapLimit: number | null;
  domNodes: number;
  // Growth of domNodes since the baseline captured when the toolbar opened.
  domNodesDelta: number;
  queryCacheCount: number;
  // Growth of queryCacheCount since that same baseline.
  queryCacheDelta: number;
};

type RawSamples = Omit<RuntimeSamples, "domNodesDelta" | "queryCacheDelta">;

type PerfWithMemory = Performance & {
  memory?: { usedJSHeapSize: number; totalJSHeapSize: number; jsHeapSizeLimit: number };
};

function readSamples(qc: QueryClient): RawSamples {
  const mem = (performance as PerfWithMemory).memory;
  return {
    heapUsed: mem ? mem.usedJSHeapSize : null,
    heapTotal: mem ? mem.totalJSHeapSize : null,
    heapLimit: mem ? mem.jsHeapSizeLimit : null,
    domNodes: typeof document !== "undefined" ? document.getElementsByTagName("*").length : 0,
    queryCacheCount: qc.getQueryCache().getAll().length,
  };
}

// useRuntimeSamples polls heap usage (Chromium-only performance.memory), live
// DOM node count, and React Query cache size once per second while active. It
// also reports each count's growth from a baseline captured on the first sample
// (i.e. when the toolbar opened), so a steadily climbing count — the cheap,
// cross-browser leak signal — is visible without any byte-precise measurement.
export function useRuntimeSamples(active: boolean): RuntimeSamples {
  const qc = useQueryClient();
  const baseline = useRef<{ domNodes: number; queryCacheCount: number } | null>(null);
  const withDelta = useCallback((raw: RawSamples): RuntimeSamples => {
    if (baseline.current === null) {
      baseline.current = { domNodes: raw.domNodes, queryCacheCount: raw.queryCacheCount };
    }
    return {
      ...raw,
      domNodesDelta: raw.domNodes - baseline.current.domNodes,
      queryCacheDelta: raw.queryCacheCount - baseline.current.queryCacheCount,
    };
  }, []);
  const [samples, setSamples] = useState<RuntimeSamples>(() => withDelta(readSamples(qc)));
  useEffect(() => {
    if (!active) return;
    setSamples(withDelta(readSamples(qc)));
    const id = setInterval(() => setSamples(withDelta(readSamples(qc))), 1000);
    return () => clearInterval(id);
  }, [active, qc, withDelta]);
  return samples;
}

// readAssetBytes sums the decoded body size of every resource in the
// Performance resource-timing buffer plus the navigation document — a
// cross-browser footprint *estimate*, not a heap reading. Caveats: cross-origin
// resources without a Timing-Allow-Origin header report 0, and the buffer
// evicts old entries, so treat the result as a ballpark rather than an exact
// total.
export function readAssetBytes(): number | null {
  if (typeof performance === "undefined" || typeof performance.getEntriesByType !== "function") {
    return null;
  }
  let total = 0;
  for (const e of performance.getEntriesByType("navigation")) {
    total += (e as PerformanceResourceTiming).decodedBodySize || 0;
  }
  for (const e of performance.getEntriesByType("resource")) {
    total += (e as PerformanceResourceTiming).decodedBodySize || 0;
  }
  return total;
}

// useAssetBytes snapshots readAssetBytes() each time the section becomes active.
// It deliberately does not poll: the resource buffer changes slowly and
// re-summing it every second is exactly the per-tick overhead this toolbar
// is built to avoid.
export function useAssetBytes(active: boolean): number | null {
  const [bytes, setBytes] = useState<number | null>(null);
  useEffect(() => {
    if (!active) return;
    setBytes(readAssetBytes());
  }, [active]);
  return bytes;
}

export function useInflightRequests(): number {
  return useSyncExternalStore(inflightRequests.subscribe, inflightRequests.get);
}

export function useSseStatus(): SseStatus {
  return useSyncExternalStore(sseStatus.subscribe, sseStatus.get);
}

export function formatBytes(n: number | null): string {
  if (n == null) return "n/a";
  if (n < 1024) return `${n} B`;
  const kb = n / 1024;
  if (kb < 1024) return `${kb.toFixed(0)} KB`;
  const mb = kb / 1024;
  if (mb < 1024) return `${mb.toFixed(1)} MB`;
  return `${(mb / 1024).toFixed(2)} GB`;
}
