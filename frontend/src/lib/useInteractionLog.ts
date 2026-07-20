import { useEffect, useState, useCallback, useRef } from 'react'
import { onUserMsg, onChatDelta, onChatDone, onChatError, onChatUsage, onNoteProposed, onAlertProposed } from './events'
import type { Usage } from '../api/types'

export type LogEntry = {
  id: number
  timestamp: string
  type: 'user_msg' | 'delta' | 'done' | 'error' | 'usage' | 'note_proposed' | 'alert_proposed' | 'clear'
  content?: string
  usage?: Usage
  note?: { title: string; content: string; tags: string[] }
  alert?: { message: string; fireAt: string; recurrence: string }
}

let logId = 0
const MAX_LOG = 500

export function useInteractionLog() {
  const [log, setLog] = useState<LogEntry[]>([])
  const [paused, setPaused] = useState(false)
  const pausedRef = useRef(false)

  useEffect(() => { pausedRef.current = paused }, [paused])

  const add = useCallback((entry: Omit<LogEntry, 'id' | 'timestamp'>) => {
    if (pausedRef.current) return
    setLog((prev) => {
      const next: LogEntry = { ...entry, id: ++logId, timestamp: new Date().toLocaleTimeString() }
      return prev.length >= MAX_LOG ? [...prev.slice(-MAX_LOG + 50), next] : [...prev, next]
    })
  }, [])

  useEffect(() => {
    const offUserMsg = onUserMsg((text) => add({ type: 'user_msg', content: text }))
    const offDelta = onChatDelta((delta) => add({ type: 'delta', content: delta }))
    const offDone = onChatDone((d) => add({ type: 'done', content: d.content }))
    const offErr = onChatError((msg) => add({ type: 'error', content: msg }))
    const offUsage = onChatUsage((u) => add({ type: 'usage', usage: u }))
    const offNote = onNoteProposed((n) => add({ type: 'note_proposed', note: n }))
    const offAlert = onAlertProposed((a) => add({ type: 'alert_proposed', alert: a }))
    return () => { offUserMsg(); offDelta(); offDone(); offErr(); offUsage(); offNote(); offAlert() }
  }, [add])

  const clear = useCallback(() => {
    setLog([])
  }, [])

  return { log, clear, paused, setPaused }
}
