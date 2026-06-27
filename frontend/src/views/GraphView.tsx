import { useEffect, useRef, useState } from 'react'
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCenter,
  forceCollide,
  type Simulation,
  type SimulationNodeDatum,
} from 'd3-force'
import { NotesService } from '../../bindings/gix/internal/app'
import type { GraphNode, GraphEdge } from '../../bindings/gix/internal/app'

interface SimNode extends SimulationNodeDatum {
  id: number
  title: string
  tags: string[]
  radius: number
  linkCount: number
}

interface SimLink {
  source: SimNode
  target: SimNode
}

const TAG_COLORS = ['#a78bfa', '#60a5fa', '#34d399', '#f472b6', '#fbbf24', '#fb923c', '#818cf8', '#2dd4bf']

const THEME_TEXT: Record<string, string> = {
  dark: 'rgba(255,255,255,0.75)',
  light: 'rgba(0,0,0,0.7)',
}

const THEME_TEXT_HOVER: Record<string, string> = {
  dark: '#fff',
  light: '#000',
}

const THEME_EDGE: Record<string, string> = {
  dark: 'rgba(255,255,255,0.12)',
  light: 'rgba(0,0,0,0.12)',
}

const THEME_EDGE_HOVER: Record<string, string> = {
  dark: 'rgba(255,255,255,0.5)',
  light: 'rgba(0,0,0,0.35)',
}

export function GraphView({
  lang,
  onClose,
  onSelectNote,
}: {
  lang: string
  onClose: () => void
  onSelectNote: (id: number) => void
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const simRef = useRef<Simulation<SimNode, SimLink> | null>(null)
  const nodesRef = useRef<SimNode[]>([])
  const linksRef = useRef<SimLink[]>([])
  const bgStarsRef = useRef<{ x: number; y: number; r: number; a: number }[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [hoveredId, setHoveredId] = useState<number | null>(null)
  const dragRef = useRef<{ node: SimNode | null; x: number; y: number }>({ node: null, x: 0, y: 0 })
  const transformRef = useRef({ x: 0, y: 0, scale: 1 })
  const isPanningRef = useRef(false)
  const panStartRef = useRef({ x: 0, y: 0 })
  const rafRef = useRef(0)
  const themeRef = useRef(document.documentElement.dataset.theme || 'dark')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    NotesService.GetGraphData().then((data) => {
      if (cancelled) return
      const gd = data as unknown as { nodes: GraphNode[]; edges: GraphEdge[] }
      if (!gd || !gd.nodes || gd.nodes.length === 0) {
        setLoading(false)
        return
      }

      const edgeCount = new Map<number, number>()
      gd.edges.forEach((e) => {
        edgeCount.set(e.source, (edgeCount.get(e.source) || 0) + 1)
        edgeCount.set(e.target, (edgeCount.get(e.target) || 0) + 1)
      })

      const simNodes: SimNode[] = gd.nodes.map((n) => ({
        id: n.id,
        title: n.title,
        tags: n.tags ?? [],
        radius: 5 + Math.min((edgeCount.get(n.id) || 0) * 1.5, 6),
        linkCount: edgeCount.get(n.id) || 0,
      }))

      const nodeMap = new Map(simNodes.map((n) => [n.id, n]))
      const simLinks: SimLink[] = gd.edges
        .filter((e) => nodeMap.has(e.source) && nodeMap.has(e.target))
        .map((e) => ({ source: nodeMap.get(e.source)!, target: nodeMap.get(e.target)! }))

      nodesRef.current = simNodes
      linksRef.current = simLinks

      const container = containerRef.current
      const cx = container ? container.clientWidth / 2 : 300
      const cy = container ? container.clientHeight / 2 : 200

      simNodes.forEach((n) => {
        n.x = cx + (Math.random() - 0.5) * Math.max(cx, cy) * 0.8
        n.y = cy + (Math.random() - 0.5) * Math.max(cx, cy) * 0.8
      })

      const sim = forceSimulation(simNodes)
        .force('link', forceLink<SimNode, SimLink>(simLinks).distance(100).strength(0.4))
        .force('charge', forceManyBody().strength(-250))
        .force('center', forceCenter(cx, cy))
        .force('collide', forceCollide((n) => n.radius + 6))
        .alphaDecay(0.02)
        .velocityDecay(0.3)
        .on('tick', () => { if (!cancelled) draw() })

      sim.alpha(1).restart()
      simRef.current = sim
      setLoading(false)
    }).catch(() => {
      if (!cancelled) { setError('Failed to load graph data'); setLoading(false) }
    })

    return () => { cancelled = true; simRef.current?.stop() }
  }, [])

  // Generate background stars
  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    const w = container.clientWidth
    const h = container.clientHeight
    const stars: { x: number; y: number; r: number; a: number }[] = []
    const count = Math.floor((w * h) / 8000)
    for (let i = 0; i < count; i++) {
      stars.push({
        x: Math.random() * w,
        y: Math.random() * h,
        r: Math.random() * 1.2 + 0.3,
        a: Math.random() * 0.4 + 0.1,
      })
    }
    bgStarsRef.current = stars
  }, [])

  // Resize handler
  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    const ro = new ResizeObserver(() => {
      const sim = simRef.current
      if (!sim) return
      const w = container.clientWidth
      const h = container.clientHeight
      sim.force('center', forceCenter(w / 2, h / 2))
      if (sim.alpha() < 0.3) sim.alpha(0.3).restart()
    })
    ro.observe(container)
    return () => ro.disconnect()
  }, [])

  // Non-passive wheel listener
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const handler = (e: WheelEvent) => {
      e.preventDefault()
      const t = transformRef.current
      const delta = -e.deltaY * 0.001
      const newScale = Math.max(0.2, Math.min(3, t.scale * (1 + delta)))
      const rect = canvas.getBoundingClientRect()
      const mx = e.clientX - rect.left
      const my = e.clientY - rect.top
      const scaleRatio = newScale / t.scale
      t.x = mx - (mx - t.x) * scaleRatio
      t.y = my - (my - t.y) * scaleRatio
      t.scale = newScale
      cancelAnimationFrame(rafRef.current)
      rafRef.current = requestAnimationFrame(draw)
    }
    canvas.addEventListener('wheel', handler, { passive: false })
    return () => canvas.removeEventListener('wheel', handler)
  }, [])

  const draw = () => {
    const canvas = canvasRef.current
    const container = containerRef.current
    if (!canvas || !container) return
    const w = container.clientWidth
    const h = container.clientHeight
    const dpr = window.devicePixelRatio || 1

    if (canvas.width !== w * dpr || canvas.height !== h * dpr) {
      canvas.width = w * dpr
      canvas.height = h * dpr
      canvas.style.width = w + 'px'
      canvas.style.height = h + 'px'
    }

    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    ctx.clearRect(0, 0, w, h)

    const theme = themeRef.current
    const t = transformRef.current

    // Draw background stars (only when not zoomed in too much)
    if (t.scale < 1.5) {
      bgStarsRef.current.forEach((star) => {
        ctx.fillStyle = `rgba(255,255,255,${star.a})`
        ctx.beginPath()
        ctx.arc(star.x, star.y, star.r, 0, Math.PI * 2)
        ctx.fill()
      })
    }

    ctx.translate(t.x, t.y)
    ctx.scale(t.scale, t.scale)

    const nodes = nodesRef.current
    const links = linksRef.current
    const hovered = hoveredId

    // Draw edges
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

    // Draw nodes
    nodes.forEach((n) => {
      const isH = n.id === hovered
      const cx = n.x!
      const cy = n.y!
      const r = isH ? n.radius + 3 : n.radius

      // Glow on hover
      if (isH) {
        const g = ctx.createRadialGradient(cx, cy, 0, cx, cy, r * 4)
        g.addColorStop(0, 'rgba(167,139,250,0.25)')
        g.addColorStop(1, 'rgba(167,139,250,0)')
        ctx.fillStyle = g
        ctx.beginPath()
        ctx.arc(cx, cy, r * 4, 0, Math.PI * 2)
        ctx.fill()
      }

      // Node circle
      const tagIdx = n.tags.length > 0
        ? Math.abs(hashStr(n.tags[0])) % TAG_COLORS.length
        : n.id % TAG_COLORS.length
      ctx.fillStyle = TAG_COLORS[tagIdx]
      ctx.beginPath()
      ctx.arc(cx, cy, r, 0, Math.PI * 2)
      ctx.fill()

      // Inner highlight
      ctx.fillStyle = theme === 'dark' ? 'rgba(255,255,255,0.5)' : 'rgba(255,255,255,0.7)'
      ctx.beginPath()
      ctx.arc(cx - r * 0.2, cy - r * 0.2, r * 0.3, 0, Math.PI * 2)
      ctx.fill()

      // Label
      if (isH || t.scale > 0.7) {
        ctx.fillStyle = isH ? THEME_TEXT_HOVER[theme] : THEME_TEXT[theme]
        ctx.font = isH ? 'bold 11px system-ui, sans-serif' : '10px system-ui, sans-serif'
        ctx.textAlign = 'center'
        ctx.textBaseline = 'top'
        const label = n.title.length > 22 ? n.title.slice(0, 22) + '…' : n.title
        const labelY = cy + r + 4
        // Background pill for readability
        const metrics = ctx.measureText(label)
        const pw = metrics.width + 10
        const ph = 16
        ctx.fillStyle = theme === 'dark' ? 'rgba(0,0,0,0.55)' : 'rgba(255,255,255,0.75)'
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

  const getCanvasPos = (e: React.MouseEvent) => {
    const rect = canvasRef.current!.getBoundingClientRect()
    const t = transformRef.current
    return {
      x: (e.clientX - rect.left - t.x) / t.scale,
      y: (e.clientY - rect.top - t.y) / t.scale,
    }
  }

  const findNode = (x: number, y: number): SimNode | null => {
    for (let i = nodesRef.current.length - 1; i >= 0; i--) {
      const n = nodesRef.current[i]
      const dx = x - n.x!
      const dy = y - n.y!
      const r = n.id === hoveredId ? n.radius + 3 : n.radius
      if (dx * dx + dy * dy <= r * r) return n
    }
    return null
  }

  const scheduleDraw = () => {
    cancelAnimationFrame(rafRef.current)
    rafRef.current = requestAnimationFrame(draw)
  }

  const onMouseDown = (e: React.MouseEvent) => {
    if (e.button !== 0) return
    const pos = getCanvasPos(e)
    const node = findNode(pos.x, pos.y)
    if (node) {
      dragRef.current = { node, x: e.clientX, y: e.clientY }
      simRef.current?.alphaTarget(0.3).restart()
    } else {
      isPanningRef.current = true
      panStartRef.current = { x: e.clientX - transformRef.current.x, y: e.clientY - transformRef.current.y }
    }
  }

  const onMouseMove = (e: React.MouseEvent) => {
    const drag = dragRef.current
    if (drag.node) {
      const t = transformRef.current
      drag.node.x! += (e.clientX - drag.x) / t.scale
      drag.node.y! += (e.clientY - drag.y) / t.scale
      drag.x = e.clientX
      drag.y = e.clientY
      return
    }
    if (isPanningRef.current) {
      transformRef.current.x = e.clientX - panStartRef.current.x
      transformRef.current.y = e.clientY - panStartRef.current.y
      scheduleDraw()
      return
    }
    const pos = getCanvasPos(e)
    const node = findNode(pos.x, pos.y)
    setHoveredId(node ? node.id : null)
    if (canvasRef.current) {
      canvasRef.current.style.cursor = node ? 'pointer' : 'grab'
    }
  }

  const onMouseUp = () => {
    if (dragRef.current.node) {
      simRef.current?.alphaTarget(0)
      dragRef.current.node = null
    }
    isPanningRef.current = false
  }

  const onClick = (e: React.MouseEvent) => {
    const pos = getCanvasPos(e)
    const node = findNode(pos.x, pos.y)
    if (node) onSelectNote(node.id)
  }

  return (
    <div className="relative flex h-full flex-col font-mono text-fg">
      <div className="flex shrink-0 items-center gap-2 border-b border-fg/8 px-3 py-2">
        <button
          onClick={onClose}
          className="flex cursor-pointer items-center gap-1 text-sm text-muted outline-none hover:text-fg"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"
            strokeLinecap="round" strokeLinejoin="round" className="size-4">
            <path d="M15 18l-6-6 6-6" />
          </svg>
          {lang === 'pt' ? 'Voltar' : 'Back'}
        </button>
        <span className="text-xs text-muted">
          {lang === 'pt' ? 'Constelação de notas' : 'Note constellation'}
          {!loading && nodesRef.current.length > 0 && (
            <span className="ml-2 opacity-50">
              · {nodesRef.current.length} {lang === 'pt' ? 'nós' : 'nodes'}
              {linksRef.current.length > 0 && ` · ${linksRef.current.length} ${lang === 'pt' ? 'conexões' : 'links'}`}
            </span>
          )}
        </span>
      </div>

      <div ref={containerRef} className="min-h-0 flex-1 overflow-hidden">
        {loading && (
          <div className="flex h-full items-center justify-center text-sm text-muted">
            {lang === 'pt' ? 'Carregando…' : 'Loading…'}
          </div>
        )}
        {error && (
          <div className="flex h-full items-center justify-center text-sm text-red-400">{error}</div>
        )}
        <canvas
          ref={canvasRef}
          className={`block h-full w-full ${loading || error ? 'hidden' : ''}`}
          onMouseDown={onMouseDown}
          onMouseMove={onMouseMove}
          onMouseUp={onMouseUp}
          onMouseLeave={onMouseUp}
          onClick={onClick}
        />
        {!loading && !error && nodesRef.current.length === 0 && (
          <div className="flex h-full items-center justify-center px-4 text-center text-sm text-muted">
            <div>
              <p className="mb-2">{lang === 'pt' ? 'Nenhuma nota com links.' : 'No linked notes.'}</p>
              <p className="opacity-60">
                {lang === 'pt'
                  ? 'Notas com a mesma tag aparecem conectadas.'
                  : 'Notes sharing a tag are connected.'}
              </p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function hashStr(s: string): number {
  let hash = 0
  for (let i = 0; i < s.length; i++) {
    hash = ((hash << 5) - hash) + s.charCodeAt(i)
    hash |= 0
  }
  return hash
}
