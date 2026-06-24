// The interaction primitive: how a command asks the user to choose from a list
// or to type a value. Kept free of React so the shell can render it and the
// pure navigation logic can be unit-tested.

// A selectable option in a `choose` card.
export type Choice = { label: string; value: string; hint?: string }

// An active request awaiting the user. `choose` renders an options card; `prompt`
// puts the bar into input mode. Exactly one is active at a time (see App.tsx).
export type Interaction =
  | { kind: 'choose'; title: string; choices: Choice[]; selected: number }
  | { kind: 'prompt'; title: string; value: string; placeholder?: string; error?: string }

// Pure: moves the highlighted index by `dir` (-1 up / +1 down) over `len`
// options, wrapping around the ends. Returns `current` unchanged for an empty
// list. Kept separate from the card component so the navigation rule is testable.
export function moveSelection(len: number, current: number, dir: number): number {
  if (len <= 0) return current
  return (((current + dir) % len) + len) % len
}
