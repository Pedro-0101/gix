import { useCallback, useEffect, useRef } from 'react'
import { moveSlider } from '../commands/interaction'

// A reusable bounded-number control. ←/→ nudge by `step` (↑/↓ too), Home/End jump
// to the ends, and click/drag on the track sets the value directly. Used both in
// the /config palette flow (where it self-focuses and Enter commits) and inline in
// the Settings form. Purely presentational state lives in the parent via `value`/
// `onChange`; the bounds rule itself is the tested `moveSlider` helper.
export function Slider({
  value,
  min,
  max,
  step,
  onChange,
  onCommit,
  autoFocus,
  ariaLabel,
}: {
  value: number
  min: number
  max: number
  step: number
  onChange: (v: number) => void
  onCommit?: () => void
  autoFocus?: boolean
  ariaLabel?: string
}) {
  const trackRef = useRef<HTMLDivElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)

  const pct = max > min ? ((value - min) / (max - min)) * 100 : 0

  useEffect(() => {
    if (autoFocus) rootRef.current?.focus()
  }, [autoFocus])

  // Map a clientX over the track to the nearest stepped value within bounds.
  const setFromClientX = useCallback((clientX: number) => {
    const el = trackRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    if (rect.width <= 0) return
    const ratio = Math.min(1, Math.max(0, (clientX - rect.left) / rect.width))
    const raw = min + ratio * (max - min)
    const stepped = Math.round((raw - min) / step) * step + min
    const clamped = Math.min(max, Math.max(min, stepped))
    if (clamped !== value) onChange(clamped)
  }, [min, max, step, value, onChange])

  // Pointer drag: capture moves on the window so the thumb keeps tracking even if
  // the cursor leaves the track, and release anywhere ends the drag.
  useEffect(() => {
    if (!dragging.current) return
    const move = (e: PointerEvent) => setFromClientX(e.clientX)
    const up = () => { dragging.current = false }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up, { once: true })
    return () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up) }
  })

  const onKeyDown = (e: React.KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowRight':
      case 'ArrowUp':
        e.preventDefault(); onChange(moveSlider(value, 1, step, min, max)); break
      case 'ArrowLeft':
      case 'ArrowDown':
        e.preventDefault(); onChange(moveSlider(value, -1, step, min, max)); break
      case 'Home':
        e.preventDefault(); onChange(min); break
      case 'End':
        e.preventDefault(); onChange(max); break
      case 'Enter':
        if (onCommit) { e.preventDefault(); e.stopPropagation(); onCommit() }
        break
    }
  }

  return (
    <div className="flex items-center gap-3 [--wails-draggable:no-drag]">
      <div
        ref={rootRef}
        role="slider"
        tabIndex={0}
        aria-label={ariaLabel}
        aria-valuemin={min}
        aria-valuemax={max}
        aria-valuenow={value}
        onKeyDown={onKeyDown}
        className="group relative flex-1 cursor-pointer py-2 outline-none"
        onPointerDown={(e) => {
          e.preventDefault()
          rootRef.current?.focus()
          dragging.current = true
          setFromClientX(e.clientX)
        }}
      >
        {/* Track */}
        <div ref={trackRef} className="h-1.5 w-full rounded-full bg-bubble shadow-[inset_0_0_0_1px_var(--shell-border)]">
          {/* Fill */}
          <div
            className="h-full rounded-full bg-accent transition-[width] duration-75 ease-out"
            style={{ width: `${pct}%` }}
          />
        </div>
        {/* Thumb */}
        <div
          className={
            'pointer-events-none absolute top-1/2 size-4 -translate-x-1/2 -translate-y-1/2 rounded-full bg-accent ' +
            'shadow-[0_1px_3px_rgba(0,0,0,0.3),0_0_0_2px_var(--shell-bg)] ' +
            'transition-[width,height,box-shadow] duration-100 ease-out ' +
            'group-hover:shadow-[0_1px_3px_rgba(0,0,0,0.3),0_0_0_3px_var(--shell-bg)] ' +
            'group-focus:shadow-[0_1px_3px_rgba(0,0,0,0.3),0_0_0_2px_var(--shell-bg),0_0_0_4px_var(--ring-focus)]'
          }
          style={{ left: `${pct}%` }}
        />
      </div>
      <span className="w-10 shrink-0 text-right font-mono text-sm tabular-nums text-fg">{value}</span>
    </div>
  )
}
