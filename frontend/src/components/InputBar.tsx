import { motion } from "motion/react"
import { type KeyboardEvent as ReactKeyboardEvent, type RefObject } from "react"
import type { Interaction } from "../commands/interaction"
import type { BarHighlight } from "../commands/highlight"
import { tr } from "../i18n"
import { ArrowIcon, PromptIcon, Spinner } from "./icons"

type Props = {
  lang: string
  input: string
  setInput: (v: string) => void
  streaming: boolean
  interaction: Interaction | null
  bar: BarHighlight
  barRef: RefObject<HTMLDivElement>
  taRef: RefObject<HTMLTextAreaElement>
  overlayRef: RefObject<HTMLDivElement>
  onDetachHistory: () => void
  onKeyDown: (e: ReactKeyboardEvent<HTMLTextAreaElement>) => void
  onSubmit: () => void
}

const item = {
  hidden: { opacity: 0, y: 8, filter: "blur(4px)" },
  show: { opacity: 1, y: 0, filter: "blur(0px)", transition: { type: "spring" as const, duration: 0.3, bounce: 0 } },
}

// InputBar is the always-visible prompt bar (and the window's drag handle): a
// transparent textarea over an overlay that paints the command token and the
// ghost name completion. Purely presentational — all behavior is delegated up.
export function InputBar({
  lang, input, setInput, streaming, interaction, bar,
  barRef, taRef, overlayRef, onDetachHistory, onKeyDown, onSubmit,
}: Props) {
  return (
    <motion.div
      ref={barRef}
      variants={item}
      className="flex shrink-0 items-end gap-2 px-3 py-2.5 [--wails-draggable:drag]"
    >
      <span className="grid size-7 shrink-0 place-items-center self-center text-muted [--wails-draggable:no-drag]">
        {streaming ? <Spinner /> : <PromptIcon />}
      </span>
      {/* The textarea text is transparent; the overlay behind it paints the
          command token (accent) and the ghost name completion (muted). They
          share identical typography so the layers line up exactly. */}
      <div className="relative flex-1 self-center [--wails-draggable:no-drag]">
        <div
          ref={overlayRef}
          aria-hidden
          className="pointer-events-none absolute inset-0 max-h-[132px] overflow-hidden whitespace-pre-wrap break-words px-0 py-1 text-[15px] leading-relaxed"
        >
          {input.length > 0 && (bar.isCommand ? (
            <><span className="text-accent">{input}</span><span className="text-muted/50">{bar.completion}</span></>
          ) : (
            <span className="text-fg">{input}</span>
          ))}
        </div>
        <textarea
          ref={taRef}
          className="relative block max-h-[132px] w-full resize-none bg-transparent px-0 py-1 text-[15px] leading-relaxed text-transparent caret-[var(--color-fg)] outline-none placeholder:text-muted/70 disabled:cursor-not-allowed"
          rows={1}
          value={input}
          spellCheck={false}
          autoCorrect="off"
          autoCapitalize="off"
          disabled={interaction?.kind === "choose" || interaction?.kind === "slider"}
          placeholder={interaction?.kind === "prompt"
            ? (interaction.placeholder ?? tr(lang, "enter_value"))
            : tr(lang, "placeholder")}
          onChange={(e) => { onDetachHistory(); setInput(e.target.value) }}
          onScroll={(e) => { if (overlayRef.current) overlayRef.current.scrollTop = e.currentTarget.scrollTop }}
          onKeyDown={onKeyDown}
        />
      </div>
      <button
        onClick={onSubmit}
        disabled={interaction?.kind === "choose" || (interaction == null && (streaming || !input.trim()))}
        aria-label={tr(lang, "placeholder")}
        className="grid size-8 shrink-0 self-center place-items-center rounded-field bg-accent text-white outline-none transition-[scale,opacity] duration-150 ease-out [--wails-draggable:no-drag] hover:brightness-110 active:not-disabled:scale-[0.96] disabled:opacity-40 focus-visible:shadow-[0_0_0_2px_var(--shell-bg),0_0_0_4px_var(--ring-focus)]"
      >
        <ArrowIcon />
      </button>
    </motion.div>
  )
}
