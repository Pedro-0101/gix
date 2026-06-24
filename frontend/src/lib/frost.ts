// The frost overlay painted over the native Windows Acrylic backdrop. Wails can't
// tint the Acrylic from Go, so the palette's translucency is a CSS layer on top:
// a white veil whose strength follows the Opacity setting (0–100).
//
// Both themes lift toward white (never a black overlay): on light the veil is a
// bright frost; on dark it's a subtle glassy lift. Higher Opacity always means a
// more opaque, more readable pane — and dark mode no longer reads as a near-black
// slab over the backdrop.

// Peak alpha of the veil at Opacity 100, per theme. Dark stays glassy so the
// Acrylic blur shows through; light goes brighter for contrast.
const PEAK = { light: 0.55, dark: 0.18 } as const

const clamp = (n: number, min: number, max: number) => Math.min(max, Math.max(min, n))

// frostColor maps the theme + Opacity (0–100) to the shell's background colour.
export function frostColor(theme: string, opacity: number): string {
  const o = clamp(opacity, 0, 100) / 100
  const peak = theme === 'dark' ? PEAK.dark : PEAK.light
  const alpha = Math.round(o * peak * 1000) / 1000
  return `rgba(255, 255, 255, ${alpha})`
}
