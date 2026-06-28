import { useEffect, useState } from "react"
import { onChatDelta, onChatDone, onChatError, onChatUsage } from "./events"
import { tr } from "../i18n"
import type { Msg, Usage } from "../types"

// useChat owns the conversation state (messages, streaming flag, token usage)
// and wires the backend streaming events into it. Because the wiring uses this
// hook's own setters, the shell stays free of the streaming plumbing.
export function useChat(lang: string) {
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [usage, setUsage] = useState<Usage | null>(null)
  const [streaming, setStreaming] = useState(false)

  useEffect(() => {
    const offDelta = onChatDelta((delta) => {
      setMsgs((m) => {
        const copy = [...m]
        const i = copy.length - 1
        const last = copy[i]
        if (last && last.role === "assistant") copy[i] = { ...last, content: last.content + delta, pending: false }
        return copy
      })
    })
    const offDone = onChatDone((d) => {
      setStreaming(false)
      setMsgs((m) => {
        const copy = [...m]
        const i = copy.length - 1
        const last = copy[i]
        if (last && last.role === "assistant") copy[i] = { ...last, content: d.content, pending: false }
        return copy
      })
    })
    const offErr = onChatError((code) => {
      setStreaming(false)
      setMsgs((m) => {
        const copy = [...m]
        const i = copy.length - 1
        const last = copy[i]
        const text = code === "no_api_key" ? tr(lang, "no_api_key") : `${tr(lang, "error_prefix")}${code}`
        if (last && last.role === "assistant") copy[i] = { ...last, content: text, pending: false, instant: true }
        return copy
      })
    })
    const offUsage = onChatUsage((u) => setUsage(u))
    return () => {
      offDelta()
      offDone()
      offErr()
      offUsage()
    }
  }, [lang])

  return { msgs, setMsgs, usage, setUsage, streaming, setStreaming }
}
