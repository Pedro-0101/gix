import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCenter,
  forceCollide,
} from "d3-force"
import type { GraphNode, GraphEdge } from "../../../bindings/gix/internal/app"
import type { SimLink, SimNode, Star } from "./types"

export type GraphData = { nodes: GraphNode[]; edges: GraphEdge[] }

// buildSimulation turns the backend graph into a configured d3-force simulation:
// it sizes nodes by degree, seeds positions around (cx, cy), and wires the
// forces. The caller attaches the tick handler and restarts it.
export function buildSimulation(gd: GraphData, cx: number, cy: number) {
  const edgeCount = new Map<number, number>()
  gd.edges.forEach((e) => {
    edgeCount.set(e.source, (edgeCount.get(e.source) || 0) + 1)
    edgeCount.set(e.target, (edgeCount.get(e.target) || 0) + 1)
  })

  const nodes: SimNode[] = gd.nodes.map((n) => ({
    id: n.id,
    title: n.title,
    tags: n.tags ?? [],
    radius: 5 + Math.min((edgeCount.get(n.id) || 0) * 1.5, 6),
    linkCount: edgeCount.get(n.id) || 0,
  }))

  const nodeMap = new Map(nodes.map((n) => [n.id, n]))
  const links: SimLink[] = gd.edges
    .filter((e) => nodeMap.has(e.source) && nodeMap.has(e.target))
    .map((e) => ({ source: nodeMap.get(e.source)!, target: nodeMap.get(e.target)! }))

  nodes.forEach((n) => {
    n.x = cx + (Math.random() - 0.5) * Math.max(cx, cy) * 0.8
    n.y = cy + (Math.random() - 0.5) * Math.max(cx, cy) * 0.8
  })

  const sim = forceSimulation(nodes)
    .force("link", forceLink<SimNode, SimLink>(links).distance(100).strength(0.4))
    .force("charge", forceManyBody().strength(-250))
    .force("center", forceCenter(cx, cy))
    .force("collide", forceCollide((n) => n.radius + 6))
    .alphaDecay(0.02)
    .velocityDecay(0.3)

  return { nodes, links, sim }
}

// makeStars scatters background stars proportional to the canvas area.
export function makeStars(w: number, h: number): Star[] {
  const stars: Star[] = []
  const count = Math.floor((w * h) / 8000)
  for (let i = 0; i < count; i++) {
    stars.push({
      x: Math.random() * w,
      y: Math.random() * h,
      r: Math.random() * 1.2 + 0.3,
      a: Math.random() * 0.4 + 0.1,
    })
  }
  return stars
}
