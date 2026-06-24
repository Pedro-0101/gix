// Reveal engine: desacopla o texto que chega (target) do texto exibido (shown).
// Ritmo de catch-up exponencial — um só formato cobre stream e drain.

import { useEffect, useRef, useState } from 'react'

export const BASE_CPS = 80 // piso de caracteres/segundo durante o stream
export const TAU = 0.4     // constante de tempo (s) da aproximação ao alvo

// nextShown avança o cursor após um frame de dtSec segundos.
export function nextShown(shown: number, targetLen: number, dtSec: number, done: boolean): number {
  if (shown >= targetLen) return targetLen
  const backlog = targetLen - shown
  const step = Math.max(BASE_CPS, backlog / TAU) * dtSec
  const advanced = shown + step
  if (advanced >= targetLen) return targetLen
  if (done && targetLen - advanced < 1) return targetLen
  return advanced
}

// useReveal avança um cursor por requestAnimationFrame até alcançar target.length.
// O cursor real é float (acúmulo sub-caractere); expõe Math.floor para exibir.
export function useReveal(
  target: string,
  opts: { done: boolean; resetKey: number },
): { shown: number; revealing: boolean } {
  const { done, resetKey } = opts
  const cursorRef = useRef(0)
  const [shown, setShown] = useState(0)

  // Reset do cursor a cada novo envio.
  useEffect(() => {
    cursorRef.current = 0
    setShown(0)
  }, [resetKey])

  const targetLen = target.length

  useEffect(() => {
    if (cursorRef.current >= targetLen) {
      if (shown !== targetLen) setShown(targetLen)
      return
    }
    let raf = 0
    let prev = performance.now()
    const tick = (now: number) => {
      const dt = Math.min((now - prev) / 1000, 0.05) // clamp p/ abas em background
      prev = now
      cursorRef.current = nextShown(cursorRef.current, targetLen, dt, done)
      const floored = Math.floor(cursorRef.current)
      setShown((s) => (s !== floored ? floored : s))
      if (cursorRef.current < targetLen) raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [targetLen, done, shown])

  return { shown: Math.min(shown, targetLen), revealing: !done || shown < targetLen }
}
