import { useCallback, useEffect, useRef, useState, type Dispatch, type RefObject, type SetStateAction } from "react"
import { moveSelection, type Choice, type Interaction } from "../commands/interaction"
import type { View } from "../commands/types"
import type { Msg } from "../types"

type Opts = {
  input: string
  setInput: (v: string) => void
  setMsgs: Dispatch<SetStateAction<Msg[]>>
  setView: (v: View) => void
  taRef: RefObject<HTMLTextAreaElement | null>
}

// useInteraction owns the modal interaction system (the options card, input
// prompt and slider that commands drive via choose/prompt/slider). It keeps the
// active interaction, its promise resolver and the prompt validator, exposes the
// three async builders for CommandContext, and the finalizers the bar/cards call.
//
// Every returned handler is memoized: App wires several of them into effect/
// context dependency arrays, so stable identities keep those effects from
// re-subscribing on every render. Deps list only the reactive values each
// handler reads (React guarantees setter/ref identities, so they're omitted).
export function useInteraction({ input, setInput, setMsgs, setView, taRef }: Opts) {
  // The active interaction (options card or input prompt), or null. Its promise
  // resolver and the prompt validator live in refs (they don't affect render).
  const [interaction, setInteraction] = useState<Interaction | null>(null)
  const resolverRef = useRef<((v: string | null) => void) | null>(null)
  const validateRef = useRef<((v: string) => string | null) | undefined>(undefined)

  const choose = useCallback((req: { title: string; choices: Choice[]; silent?: boolean }) =>
    new Promise<string | null>((resolve) => {
      resolverRef.current = resolve
      setView("chat")
      setInteraction({ kind: "choose", title: req.title, choices: req.choices, selected: 0, silent: req.silent })
    }), [setView])

  const prompt = useCallback((req: { title: string; placeholder?: string; validate?: (v: string) => string | null }) =>
    new Promise<string | null>((resolve) => {
      resolverRef.current = resolve
      validateRef.current = req.validate
      setView("chat")
      setInput("")
      setInteraction({ kind: "prompt", title: req.title, value: "", placeholder: req.placeholder })
      requestAnimationFrame(() => taRef.current?.focus())
    }), [setView, setInput, taRef])

  const slider = useCallback((req: { title: string; value: number; min: number; max: number; step: number }) =>
    new Promise<string | null>((resolve) => {
      resolverRef.current = resolve
      setView("chat")
      setInteraction({ kind: "slider", title: req.title, value: req.value, min: req.min, max: req.max, step: req.step })
    }), [setView])

  // Finalize the active `choose`: record the pick as an inert message and resolve.
  const pickChoice = useCallback((index: number) => {
    if (interaction?.kind !== "choose") return
    const choice = interaction.choices[index]
    if (!choice) return
    const resolve = resolverRef.current
    resolverRef.current = null
    if (!interaction.silent) {
      setMsgs((m) => [...m, { role: "choice", title: interaction.title, chosenLabel: choice.label }])
    }
    setInteraction(null)
    resolve?.(choice.value)
    requestAnimationFrame(() => taRef.current?.focus())
  }, [interaction, setMsgs, taRef])

  // Finalize the active `prompt`: validate the typed value; on success record it
  // and resolve, otherwise show the error and keep the bar open.
  const submitPrompt = useCallback(() => {
    if (interaction?.kind !== "prompt") return
    const value = input
    const err = validateRef.current?.(value) ?? null
    if (err) {
      setInteraction({ ...interaction, error: err })
      return
    }
    const resolve = resolverRef.current
    resolverRef.current = null
    validateRef.current = undefined
    setMsgs((m) => [...m, { role: "choice", title: interaction.title, chosenLabel: value || "—" }])
    setInteraction(null)
    setInput("")
    resolve?.(value)
  }, [interaction, input, setMsgs, setInput])

  // Finalize the active `slider`: record the chosen number as an inert message and
  // resolve with its string form (config.apply coerces it back to a number).
  const submitSlider = useCallback(() => {
    if (interaction?.kind !== "slider") return
    const value = interaction.value
    const resolve = resolverRef.current
    resolverRef.current = null
    setMsgs((m) => [...m, { role: "choice", title: interaction.title, chosenLabel: String(value) }])
    setInteraction(null)
    resolve?.(String(value))
    requestAnimationFrame(() => taRef.current?.focus())
  }, [interaction, setMsgs, taRef])

  // Abandon any active interaction (Esc), resolving its promise with null. Return
  // focus to the bar (it was disabled during a choose) so the user can keep typing.
  const cancelInteraction = useCallback(() => {
    const resolve = resolverRef.current
    resolverRef.current = null
    validateRef.current = undefined
    setInteraction(null)
    setInput("")
    resolve?.(null)
    requestAnimationFrame(() => taRef.current?.focus())
  }, [setInput, taRef])

  // Reset on window show: drop any pending interaction and resolve it with null.
  const reset = useCallback(() => {
    resolverRef.current?.(null)
    resolverRef.current = null
    validateRef.current = undefined
    setInteraction(null)
  }, [])

  // Keyboard while an interaction is active. Capture phase so Esc/Enter here win
  // over the global double-Esc-to-hide handler and the bar's send-on-Enter.
  useEffect(() => {
    if (!interaction) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault()
        e.stopPropagation()
        cancelInteraction()
        return
      }
      if (interaction.kind !== "choose") return
      if (e.key === "ArrowDown") {
        e.preventDefault()
        setInteraction({ ...interaction, selected: moveSelection(interaction.choices.length, interaction.selected, 1) })
      } else if (e.key === "ArrowUp") {
        e.preventDefault()
        setInteraction({ ...interaction, selected: moveSelection(interaction.choices.length, interaction.selected, -1) })
      } else if (e.key === "Enter") {
        e.preventDefault()
        e.stopPropagation()
        pickChoice(interaction.selected)
      }
    }
    window.addEventListener("keydown", onKey, true)
    return () => window.removeEventListener("keydown", onKey, true)
  }, [interaction, cancelInteraction, pickChoice])

  return { interaction, setInteraction, choose, prompt, slider, pickChoice, submitPrompt, submitSlider, cancelInteraction, reset }
}
