// Reveal engine: desacopla o texto que chega (target) do texto exibido (shown).
// Ritmo de catch-up exponencial — um só formato cobre stream e drain — com um
// amortecedor de velocidade por cima, para que as rajadas irregulares do stream
// da IA não virem solavancos de ritmo no card.

import { useEffect, useRef, useState } from 'react'

export const BASE_CPS = 80   // piso de caracteres/segundo durante o stream
export const TAU = 0.4       // constante de tempo (s) da aproximação ao alvo
export const TAU_VEL = 0.22  // constante de tempo (s) do amortecedor de velocidade

// nextReveal avança o cursor e a velocidade após um frame de dtSec segundos.
//
// Modelo em duas camadas:
//  1. velocidade-alvo = catch-up proporcional ao backlog (texto recebido mas
//     ainda não exibido), com um piso BASE_CPS durante o stream;
//  2. a velocidade exibida persegue essa velocidade-alvo por um filtro
//     passa-baixa (constante de tempo TAU_VEL), em vez de saltar para ela. Esse
//     é o amortecedor: quando a IA despeja um burst, o backlog dispara, mas a
//     velocidade sobe/desce suavemente — o ritmo percebido fica constante.
//
// O cursor (shown) e a velocidade (vel) são floats acumulados entre frames.
export function nextReveal(
  shown: number,
  vel: number,
  targetLen: number,
  dtSec: number,
  done: boolean,
): { shown: number; vel: number } {
  if (shown >= targetLen) return { shown: targetLen, vel }
  const backlog = targetLen - shown
  const targetVel = Math.max(BASE_CPS, backlog / TAU)
  // Suavização exponencial da velocidade rumo à velocidade-alvo (o amortecedor).
  const a = 1 - Math.exp(-dtSec / TAU_VEL)
  const nextVel = vel + (targetVel - vel) * a
  const advanced = shown + nextVel * dtSec
  if (advanced >= targetLen) return { shown: targetLen, vel: nextVel }
  if (done && targetLen - advanced < 1) return { shown: targetLen, vel: nextVel }
  return { shown: advanced, vel: nextVel }
}

// useReveal avança um cursor por requestAnimationFrame até alcançar target.length.
// O cursor real é float (acúmulo sub-caractere); expõe Math.floor para exibir.
export function useReveal(
  target: string,
  opts: { done: boolean; resetKey: number },
): { shown: number; revealing: boolean } {
  const { done, resetKey } = opts
  const cursorRef = useRef(0)
  const velRef = useRef(0)
  const [shown, setShown] = useState(0)

  // Reset do cursor e da velocidade a cada novo envio.
  useEffect(() => {
    cursorRef.current = 0
    velRef.current = 0
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
      const r = nextReveal(cursorRef.current, velRef.current, targetLen, dt, done)
      cursorRef.current = r.shown
      velRef.current = r.vel
      const floored = Math.floor(cursorRef.current)
      setShown((s) => (s !== floored ? floored : s))
      if (cursorRef.current < targetLen) raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [targetLen, done, shown])

  return { shown: Math.min(shown, targetLen), revealing: !done || shown < targetLen }
}
