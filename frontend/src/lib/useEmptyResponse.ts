import { useEffect, useRef, type Dispatch, type SetStateAction } from 'react'
import { onChatEmptyResponse } from './events'
import { tr } from '../i18n'
import type { CommandContext } from '../commands/types'
import type { Msg } from '../types'

const MAX_AUTO_RETRY = 3

type Deps = {
  setStreaming: (v: boolean) => void
  setMsgs: Dispatch<SetStateAction<Msg[]>>
  commandContextRef: { current: CommandContext }
}

export function useEmptyResponse({ setStreaming, setMsgs, commandContextRef }: Deps) {
  const deps = useRef({ setStreaming, setMsgs, commandContextRef })
  deps.current = { setStreaming, setMsgs, commandContextRef }

  useEffect(() => {
    let retryCount = 0
    let currentText = ''

    const off = onChatEmptyResponse(async (text) => {
      // Reset counter when user sends a different message
      if (text !== currentText) {
        currentText = text
        retryCount = 0
      }
      retryCount++

      const { setStreaming, setMsgs, commandContextRef } = deps.current
      const ctx = commandContextRef.current
      const { ChatService } = await import('../api/services')

      if (retryCount <= MAX_AUTO_RETRY) {
        ctx.emitSystemMessage(
          `_${tr(ctx.lang, 'empty_retrying')} (${retryCount}/${MAX_AUTO_RETRY})…_`,
        )
        // Remove the empty assistant message, add a new pending one, and resend
        setMsgs((m) => {
          const copy = [...m]
          const last = copy[copy.length - 1]
          if (last && last.role === 'assistant') copy.pop()
          copy.push({ role: 'assistant', content: '', pending: true })
          return copy
        })
        setStreaming(true)
        ChatService.send(text)
        return
      }

      // After MAX_AUTO_RETRY consecutive failures, ask the user
      const ok = await ctx.choose({
        title: tr(ctx.lang, 'empty_response_title').replace('{n}', String(retryCount)),
        choices: [
          { label: tr(ctx.lang, 'alert_yes'), value: 'yes' },
          { label: tr(ctx.lang, 'alert_no'), value: 'no' },
        ],
      })
      if (ok !== 'yes') return

      setMsgs((m) => {
        const copy = [...m]
        const last = copy[copy.length - 1]
        if (last && last.role === 'assistant') copy.pop()
        copy.push({ role: 'assistant', content: '', pending: true })
        return copy
      })
      setStreaming(true)
      ChatService.send(text)
    })

    return () => { off() }
  }, [])
}
