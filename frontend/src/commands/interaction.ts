// The interaction primitive: how a command asks the user to choose from a list
// or to type a value. Kept free of React so the shell can render it and the
// pure navigation logic can be unit-tested.

// A selectable option in a `choose` card.
export type Choice = { label: string; value: string; hint?: string }

// An active request awaiting the user. `choose` renders an options card; `prompt`
// puts the bar into input mode; `slider` adjusts a bounded number with ←/→.
// Exactly one is active at a time (see App.tsx).
export type Interaction =
  // `silent` choose cards (e.g. a navigation menu) leave no inert record behind.
  | { kind: 'choose'; title: string; choices: Choice[]; selected: number; silent?: boolean }
  | { kind: 'prompt'; title: string; value: string; placeholder?: string; error?: string }
  | { kind: 'slider'; title: string; value: number; min: number; max: number; step: number }

// Pure: moves the highlighted index by `dir` (-1 up / +1 down) over `len`
// options, wrapping around the ends. Returns `current` unchanged for an empty
// list. Kept separate from the card component so the navigation rule is testable.
export function moveSelection(len: number, current: number, dir: number): number {
  if (len <= 0) return current
  return (((current + dir) % len) + len) % len
}

// Pure: nudges a slider value by `dir` (-1 left / +1 right) steps and clamps to
// [min, max]. Unlike the wrapping list selection, the slider stops at its ends.
// Kept separate from the Slider component so the bounds rule is unit-testable.
export function moveSlider(
  value: number,
  dir: number,
  step: number,
  min: number,
  max: number,
): number {
  const next = value + dir * step
  if (next < min) return min
  if (next > max) return max
  return next
}
