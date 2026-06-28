import type { SimulationNodeDatum } from "d3-force"

export interface SimNode extends SimulationNodeDatum {
  id: number
  title: string
  tags: string[]
  radius: number
  linkCount: number
}

export interface SimLink {
  source: SimNode
  target: SimNode
}

// One twinkle in the background starfield.
export type Star = { x: number; y: number; r: number; a: number }

export type Transform = { x: number; y: number; scale: number }
