import { useEffect } from "react";

// Publishes the visual viewport's height as `--app-height` on <html> so the
// app (see `#root` in index.css) is sized to what's actually visible.
//
// `height: 100%` follows the *layout* viewport. On iOS Safari — and Android
// browsers that ignore `interactive-widget=resizes-content` — the soft
// keyboard only shrinks the *visual* viewport, so a 100%-tall app keeps its
// full height and the keyboard covers the bottom of the terminal, which is
// where the agent's prompt lives.
// See REGRESSIONS.md: "Soft keyboard covers the terminal".
export function useVisualViewportHeight(): void {
  useEffect(() => {
    const vv = window.visualViewport;
    if (!vv) return;
    const root = document.documentElement;
    let raf: number | null = null;
    const apply = () => {
      raf = null;
      // vv.height is in zoomed CSS pixels: pinch-zooming (or iOS zooming into
      // a focused input) shrinks it without anything covering the page.
      // Multiplying by the scale undoes that, leaving only what the keyboard
      // took, so a zoomed app keeps its full layout and can be panned.
      root.style.setProperty("--app-height", `${Math.round(vv.height * vv.scale)}px`);
      // iOS scrolls the layout viewport to reveal the focused input, which
      // would slide the app's header off the top of the visible area. The app
      // already fits the visual viewport, so pin it back. Not while zoomed:
      // there a non-zero offset is the user panning.
      if (vv.offsetTop !== 0 && Math.abs(vv.scale - 1) < 0.01) window.scrollTo(0, 0);
    };
    const schedule = () => {
      if (raf == null) raf = requestAnimationFrame(apply);
    };
    apply();
    vv.addEventListener("resize", schedule);
    vv.addEventListener("scroll", schedule);
    return () => {
      vv.removeEventListener("resize", schedule);
      vv.removeEventListener("scroll", schedule);
      if (raf != null) cancelAnimationFrame(raf);
      root.style.removeProperty("--app-height");
    };
  }, []);
}
