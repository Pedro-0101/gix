import type { SimLink, SimNode, Star, Transform } from "./types"

const TAG_COLORS = ["#a78bfa", "#60a5fa", "#34d399", "#f472b6", "#fbbf24", "#fb923c", "#818cf8", "#2dd4bf"]

const THEME_TEXT: Record<string, string> = {
  dark: "rgba(255,255,255,0.75)",
  light: "rgba(0,0,0,0.7)",
}
const THEME_TEXT_HOVER: Record<string, string> = { dark: "#fff", light: "#000" }
const THEME_EDGE: Record<string, string> = {
  dark: "rgba(255,255,255,0.12)",
  light: "rgba(0,0,0,0.12)",
}
const THEME_EDGE_HOVER: Record<string, string> = {
  dark: "rgba(255,255,255,0.5)",
  light: "rgba(0,0,0,0.35)",
}

function hashStr(s: string): number {
  let hash = 0
  for (let i = 0; i < s.length; i++) {
    hash = (hash << 5) - hash + s.charCodeAt(i)
    hash |= 0
  }
  return hash
}

type Scene = {
  nodes: SimNode[]
  links: SimLink[]
  transform: Transform
  theme: string
  hoveredId: number | null
  bgStars: Star[]
}

// drawGraph paints the whole scene (starfield, edges, nodes, labels) onto the
// canvas in device pixels, applying the pan/zoom transform. Pure rendering: it
// reads the scene but holds no state.
export function drawGraph(canvas: HTMLCanvasElement, container: HTMLElement, scene: Scene) {
  const w = container.clientWidth
  const h = container.clientHeight
  const dpr = window.devicePixelRatio || 1

  if (canvas.width !== w * dpr || canvas.height !== h * dpr) {
    canvas.width = w * dpr
    canvas.height = h * dpr
    canvas.style.width = w + "px"
    canvas.style.height = h + "px"
  }

  const ctx = canvas.getContext("2d")
  if (!ctx) return
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, w, h)

  const { theme, transform: t, nodes, links, hoveredId: hovered } = scene

  // Background stars (only when not zoomed in too much).
  if (t.scale < 1.5) {
    scene.bgStars.forEach((star) => {
      ctx.fillStyle = `rgba(255,255,255,${star.a})`
      ctx.beginPath()
      ctx.arc(star.x, star.y, star.r, 0, Math.PI * 2)
      ctx.fill()
    })
  }

  ctx.translate(t.x, t.y)
  ctx.scale(t.scale, t.scale)

  // Edges.
  ctx.lineWidth = 1
  links.forEach((l) => {
    const s = l.source as SimNode
    const tgt = l.target as SimNode
    const isH = hovered !== null && (s.id === hovered || tgt.id === hovered)
    if (isH) {
      ctx.strokeStyle = THEME_EDGE_HOVER[theme]
      ctx.lineWidth = 1.5
    } else {
      ctx.strokeStyle = THEME_EDGE[theme]
      ctx.lineWidth = 0.8
    }
    ctx.beginPath()
    ctx.moveTo(s.x!, s.y!)
    ctx.lineTo(tgt.x!, tgt.y!)
    ctx.stroke()
  })
  ctx.lineWidth = 1

  // Nodes.
  nodes.forEach((n) => {
    const isH = n.id === hovered
    const cx = n.x!
    const cy = n.y!
    const r = isH ? n.radius + 3 : n.radius

    // Glow on hover.
    if (isH) {
      const g = ctx.createRadialGradient(cx, cy, 0, cx, cy, r * 4)
      g.addColorStop(0, "rgba(167,139,250,0.25)")
      g.addColorStop(1, "rgba(167,139,250,0)")
      ctx.fillStyle = g
      ctx.beginPath()
      ctx.arc(cx, cy, r * 4, 0, Math.PI * 2)
      ctx.fill()
    }

    // Node circle.
    const tagIdx = n.tags.length > 0
      ? Math.abs(hashStr(n.tags[0])) % TAG_COLORS.length
      : n.id % TAG_COLORS.length
    ctx.fillStyle = TAG_COLORS[tagIdx]
    ctx.beginPath()
    ctx.arc(cx, cy, r, 0, Math.PI * 2)
    ctx.fill()

    // Inner highlight.
    ctx.fillStyle = theme === "dark" ? "rgba(255,255,255,0.5)" : "rgba(255,255,255,0.7)"
    ctx.beginPath()
    ctx.arc(cx - r * 0.2, cy - r * 0.2, r * 0.3, 0, Math.PI * 2)
    ctx.fill()

    // Label.
    if (isH || t.scale > 0.7) {
      ctx.fillStyle = isH ? THEME_TEXT_HOVER[theme] : THEME_TEXT[theme]
      ctx.font = isH ? "bold 11px system-ui, sans-serif" : "10px system-ui, sans-serif"
      ctx.textAlign = "center"
      ctx.textBaseline = "top"
      const label = n.title.length > 22 ? n.title.slice(0, 22) + "…" : n.title
      const labelY = cy + r + 4
      // Background pill for readability.
      const metrics = ctx.measureText(label)
      const pw = metrics.width + 10
      const ph = 16
      ctx.fillStyle = theme === "dark" ? "rgba(0,0,0,0.55)" : "rgba(255,255,255,0.75)"
      const rx = cx - pw / 2
      const ry = labelY - 2
      ctx.beginPath()
      ctx.roundRect(rx, ry, pw, ph, 4)
      ctx.fill()
      ctx.fillStyle = isH ? THEME_TEXT_HOVER[theme] : THEME_TEXT[theme]
      ctx.fillText(label, cx, labelY)
    }
  })
}
