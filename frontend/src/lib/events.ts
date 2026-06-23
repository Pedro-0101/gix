import { Events } from '@wailsio/runtime'

export type UsagePayload = { tokens: number; cost: number }
export type DonePayload = { content: string }

export const onChatDelta = (cb: (delta: string) => void) =>
  Events.On('chat:delta', (e) => cb(e.data as string))

export const onChatUsage = (cb: (u: UsagePayload) => void) =>
  Events.On('chat:usage', (e) => cb(e.data as UsagePayload))

export const onChatDone = (cb: (d: DonePayload) => void) =>
  Events.On('chat:done', (e) => cb(e.data as DonePayload))

export const onChatError = (cb: (msg: string) => void) =>
  Events.On('chat:error', (e) => cb(e.data as string))

// Fired by Go each time the window is shown via hotkey/tray — reset to the bar.
export const onWindowShown = (cb: () => void) =>
  Events.On('window:shown', () => cb())
