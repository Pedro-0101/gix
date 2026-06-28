import { useEffect, useRef, useState } from 'react'
import { forceCenter, type Simulation } from 'd3-force'
import { NotesService } from '../../bindings/gix/internal/app'
import { buildSimulation, makeStars, type GraphData } from './graph/simulation'
import { drawGraph } from './graph/render'
import type { SimLink, SimNode, Star } from './graph/types'

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
  const bgStarsRef = useRef<Star[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [hoveredId, setHoveredId] = useState<number | null>(null)
  const dragRef = useRef<{ node: SimNode | null; x: number; y: number }>({ node: null, x: 0, y: 0 })
  const transformRef = useRef({ x: 0, y: 0, scale: 1 })
  const isPanningRef = useRef(false)
  const panStartRef = useRef({ x: 0, y: 0 })
  const rafRef = useRef(0)
  const themeRef = useRef(document.documentElement.dataset.theme || 'dark')

  // Thin wrapper: feeds the live refs + current hover into the pure renderer.
  const draw = () => {
    const canvas = canvasRef.current
    const container = containerRef.current
    if (!canvas || !container) return
    drawGraph(canvas, container, {
      nodes: nodesRef.current,
      links: linksRef.current,
      transform: transformRef.current,
      theme: themeRef.current,
      hoveredId,
      bgStars: bgStarsRef.current,
    })
  }

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    NotesService.GetGraphData().then((data) => {
      if (cancelled) return
      const gd = data as unknown as GraphData
      if (!gd || !gd.nodes || gd.nodes.length === 0) {
        setLoading(false)
        return
      }
      const container = containerRef.current
      const cx = container ? container.clientWidth / 2 : 300
      const cy = container ? container.clientHeight / 2 : 200

      const { nodes, links, sim } = buildSimulation(gd, cx, cy)
      nodesRef.current = nodes
      linksRef.current = links
      sim.on('tick', () => { if (!cancelled) draw() })
      sim.alpha(1).restart()
      simRef.current = sim
      setLoading(false)
    }).catch(() => {
      if (!cancelled) { setError('Failed to load graph data'); setLoading(false) }
    })

    return () => { cancelled = true; simRef.current?.stop() }
  }, [])

  // Background stars.
  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    bgStarsRef.current = makeStars(container.clientWidth, container.clientHeight)
  }, [])

  // Resize handler.
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

  // Non-passive wheel listener (zoom toward the cursor).
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
