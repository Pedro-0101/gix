// Reveal engine: desacopla o texto que chega (target) do texto exibido (shown).
// Ritmo de catch-up exponencial — um só formato cobre stream e drain.

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
