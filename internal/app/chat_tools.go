package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gix/internal/ai"
)

// ChatTool is one function the chat model can call. Schema describes it to the
// model; Handle runs when the model emits a matching tool_call. Adding a new
// capability to the chat (create_note, search_notes, web_search…) is registering
// a ChatTool — the streaming loop in Send dispatches generically and never grows
// an if-per-tool.
type ChatTool interface {
	Schema() ai.Tool
	Handle(call ai.ToolCall, emit Emitter, now time.Time) ToolResult
}

// ToolResult tells the Send loop how to finish after a tool ran.
//
// A tool that recognizes and acts on its call sets Handled; the loop then stops
// looking. When the model produced no prose of its own, Placeholder is persisted
// in place of the default "(sem resposta)", and SuppressDone keeps the loop from
// emitting chat:done (used when the tool already surfaced its own UI, e.g. the
// alert-confirmation chip). When prose is present it is always persisted and a
// chat:done is always emitted, regardless of these fields.
type ToolResult struct {
	Handled      bool
	Placeholder  string
	SuppressDone bool
}

// toolRegistry holds the chat's callable tools. It is the open/closed seam: Send
// asks it for the schemas to advertise and for the outcome of any tool_call,
// instead of branching per tool name.
type toolRegistry struct {
	tools []ChatTool
}

func newToolRegistry(tools ...ChatTool) toolRegistry {
	return toolRegistry{tools: tools}
}

// defaultChatTools is the built-in tool set. Add new ChatTools here; once a tool
// needs app services, inject the registry through NewChatService instead.
func defaultChatTools() toolRegistry {
	return newToolRegistry(alertTool{})
}

func (r toolRegistry) schemas() []ai.Tool {
	if len(r.tools) == 0 {
		return nil
	}
	out := make([]ai.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Schema())
	}
	return out
}

// dispatch runs the first registered tool whose name matches a tool_call and
// that reports it handled the call. A matched-but-unhandled tool (e.g. malformed
// arguments) falls through so Send treats the turn as a normal text answer.
func (r toolRegistry) dispatch(calls []ai.ToolCall, emit Emitter, now time.Time) ToolResult {
	for _, t := range r.tools {
		if call, ok := findToolCall(calls, t.Schema().Function.Name); ok {
			if res := t.Handle(call, emit, now); res.Handled {
				return res
			}
		}
	}
	return ToolResult{}
}

func findToolCall(calls []ai.ToolCall, name string) (ai.ToolCall, bool) {
	for _, c := range calls {
		if c.Name == name {
			return c, true
		}
	}
	return ai.ToolCall{}, false
}

type alertProposedPayload struct {
	Message    string `json:"message"`
	FireAt     string `json:"fireAt"`
	Recurrence string `json:"recurrence"`
}

// alertTool lets the chat model schedule a reminder. It stores nothing itself: it
// proposes the alert to the frontend (alert:proposed), which asks the user to
// confirm before CreateProposed persists it.
type alertTool struct{}

func (alertTool) Schema() ai.Tool {
	return ai.Tool{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "create_alert",
			Description: "Agenda um lembrete/alarme quando o usuário pede para ser lembrado de algo num horário ou data. Resolva datas relativas a partir do horário local atual informado no system prompt.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},"fire_at":{"type":"string","description":"ISO 8601 com offset"},"recurrence":{"type":["object","null"]}},"required":["message","fire_at"]}`),
		},
	}
}

func (alertTool) Handle(call ai.ToolCall, emit Emitter, now time.Time) ToolResult {
	p, err := parseAlertCall(call)
	if err != nil {
		// Malformed arguments: fall through to a normal text answer.
		return ToolResult{}
	}
	if p.Message != "" && futureOrRecurring(p.FireAt, p.Recurrence, now) {
		emit("alert:proposed", p)
		return ToolResult{Handled: true, Placeholder: "(propôs um alerta)", SuppressDone: true}
	}
	// A call we can't schedule (past time / empty message): give the user
	// feedback instead of a dead-end chip.
	return ToolResult{Handled: true, Placeholder: "Não consegui agendar esse lembrete."}
}

func parseAlertCall(c ai.ToolCall) (alertProposedPayload, error) {
	var dec alertDecision
	if err := json.Unmarshal([]byte(c.Arguments), &dec); err != nil {
		return alertProposedPayload{}, err
	}
	if strings.TrimSpace(dec.FireAt) == "" {
		return alertProposedPayload{}, fmt.Errorf("empty fire_at")
	}
	return alertProposedPayload{
		Message:    strings.TrimSpace(dec.Message),
		FireAt:     strings.TrimSpace(dec.FireAt),
		Recurrence: marshalRecurrence(dec.Recurrence),
	}, nil
}
