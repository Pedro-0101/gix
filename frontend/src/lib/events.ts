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

export type AlertFiredPayload = { id: number; message: string; noteId: number | null }

// Fired by Go when an alert's time arrives — show a card if the overlay is open.
export const onAlertFired = (cb: (a: AlertFiredPayload) => void) =>
  Events.On('alert:fired', (e) => cb(e.data as AlertFiredPayload))

// Fired when the user clicks a toast body — open the alerts view focused on it.
export const onAlertOpen = (cb: (id: number) => void) =>
  Events.On('alert:open', (e) => cb((e.data as { id: number }).id))

export type AlertProposedPayload = { message: string; fireAt: string; recurrence: string }

// Fired by Go when the chat model proposes an alert (create_alert tool call) —
// the shell asks the user to confirm before scheduling it.
export const onAlertProposed = (cb: (p: AlertProposedPayload) => void) =>
  Events.On('alert:proposed', (e) => cb(e.data as AlertProposedPayload))
