import { useCallback, useEffect, useRef, type RefObject } from "react"
import { Window } from "@wailsio/runtime"

// Must match the Go side (internal/app/shell.go).
const WIDTH = 680

// useWindowFit returns a `fit` callback that resizes the OS window to match the
// content height (fixed width, anchored at the top so it grows downward — no
// resize feedback loop) and keeps a ResizeObserver attached to rootRef. The
// caller drives explicit fits via the returned callback (e.g. in a layout
// effect); the observer covers async content changes. Re-attaches on `nonce`
// change, since the keyed root remounts on every window show.
export function useWindowFit(rootRef: RefObject<HTMLDivElement | null>, nonce: number) {
  const rafRef = useRef(0)
  const fit = useCallback(() => {
    cancelAnimationFrame(rafRef.current)
    rafRef.current = requestAnimationFrame(() => {
      const el = rootRef.current
      if (!el) return
      Window.SetSize(WIDTH, Math.ceil(el.getBoundingClientRect().height))
    })
  }, [rootRef])

  useEffect(() => {
    const el = rootRef.current
    if (!el || typeof ResizeObserver === "undefined") return
    const ro = new ResizeObserver(() => fit())
    ro.observe(el)
    return () => ro.disconnect()
  }, [fit, nonce, rootRef])

  return fit
}
