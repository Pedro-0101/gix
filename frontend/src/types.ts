// Shell message types, shared between App and the extracted shell hooks/components.
export type ChatMsg = {
  role: "user" | "assistant" | "system"
  content: string
  pending?: boolean
  instant?: boolean
}
export type ChoiceMsg = { role: "choice"; title: string; chosenLabel: string }
export type Msg = ChatMsg | ChoiceMsg

// Token usage + cost shown in the chat view, set from the chat:usage event.
export type Usage = { tokens: number; cost: number }
