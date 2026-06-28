import { useCallback, useEffect, useRef, type RefObject } from "react"
import { Window } from "@wailsio/runtime"

// Must match the Go side (internal/app/shell.go).
const WIDTH = 680

// How long the animated window-resize transition lasts (ms). Longer than a
// snappy UI tap on purpose: the window grows downward as content streams in,
// and a gentler, more drawn-out glide reads as "smooth expansion" rather than
// a series of small jumps.
const ANIM_MS = 340

// useWindowFit returns a `fit` callback that smoothly animates the OS window
// height to match the content (fixed width, anchored at the top so it grows
// downward) and keeps a ResizeObserver attached to rootRef.
//
// Unlike the old version that jumped straight to the target height (flash +
// abrupt expansion), this interpolates via requestAnimationFrame with an
// easeOutCubic curve. Rapid successive fits cancel the in-flight animation
// and start a new one from the current interpolated height, so streaming
// content never fights itself.
//
// The caller drives explicit fits from a layout effect (synchronous start —
// no rAF delay before the first frame); the observer covers async content
// changes. Re-attaches on `nonce` change, since the keyed root remounts on
// every window show.
export function useWindowFit(rootRef: RefObject<HTMLDivElement | null>, nonce: number) {
  const rafRef = useRef(0)

  // Track the last-set window height so animations always start from the
  // current OS size (even if another fit was cancelled mid-flight).
  const curRef = useRef(window.innerHeight)

  const fit = useCallback(() => {
    cancelAnimationFrame(rafRef.current)
    const el = rootRef.current
    if (!el) return

    const targetH = Math.ceil(el.getBoundingClientRect().height)
    const from = curRef.current
    if (from === targetH) return

    const t0 = performance.now()

    const step = (now: number) => {
      const p = Math.min((now - t0) / ANIM_MS, 1)
      // easeOutQuart: a stronger deceleration than cubic — moves promptly off
      // the start, then settles with a long, soft tail so the final pixels of
      // growth never feel abrupt.
      const eased = 1 - Math.pow(1 - p, 4)
      const h = Math.round(from + (targetH - from) * eased)
      curRef.current = h
      Window.SetSize(WIDTH, h)
      if (p < 1) rafRef.current = requestAnimationFrame(step)
    }

    rafRef.current = requestAnimationFrame(step)
  }, [rootRef])

  useEffect(() => {
    const el = rootRef.current
    if (!el || typeof ResizeObserver === "undefined") return
    const ro = new ResizeObserver(() => fit())
    ro.observe(el)
    return () => { ro.disconnect(); cancelAnimationFrame(rafRef.current) }
  }, [fit, nonce, rootRef])

  return fit
}
