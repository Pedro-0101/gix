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

// noteProposedPayload is sent to the frontend so it can ask the user to confirm
// before the note is stored.
type noteProposedPayload struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

// alertProposedPayload is sent to the frontend so it can ask the user to confirm
// before the alert is stored.
type alertProposedPayload struct {
	Message    string `json:"message"`
	FireAt     string `json:"fireAt"`
	Recurrence string `json:"recurrence"`
}

// toolParser decodes a tool_call's JSON arguments into a proposal payload.
type toolParser func(json.RawMessage) (any, error)

// toolValidator checks whether a parsed payload is worth proposing to the user.
// For alerts this checks futureOrRecurring; for notes it checks required fields.
type toolValidator func(any, time.Time) bool

// proposalTool is a reusable ChatTool that follows the propose-then-confirm
// pattern: parse the tool call, validate, emit an event to the frontend, and
// suppress chat:done so the frontend shows a confirmation chip instead.
type proposalTool struct {
	schema             ai.Tool
	parse              toolParser
	valid              toolValidator
	eventName          string
	successPlaceholder string
	failurePlaceholder string
}

func (t *proposalTool) Schema() ai.Tool { return t.schema }

func (t *proposalTool) Handle(call ai.ToolCall, emit Emitter, now time.Time) ToolResult {
	payload, err := t.parse(json.RawMessage(call.Arguments))
	if err != nil {
		return ToolResult{}
	}
	if !t.valid(payload, now) {
		return ToolResult{Handled: true, Placeholder: t.failurePlaceholder}
	}
	emit(t.eventName, payload)
	return ToolResult{Handled: true, Placeholder: t.successPlaceholder, SuppressDone: true}
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
	return newToolRegistry(newAlertProposalTool(), newNoteProposalTool())
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

// newAlertProposalTool creates a proposalTool for the create_alert function.
func newAlertProposalTool() *proposalTool {
	return &proposalTool{
		schema: ai.Tool{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "create_alert",
				Description: "Agenda um lembrete/alarme quando o usuário pede para ser lembrado de algo num horário ou data. Resolva datas relativas a partir do horário local atual informado no system prompt.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},"fire_at":{"type":"string","description":"ISO 8601 com offset"},"recurrence":{"type":["object","null"]}},"required":["message","fire_at"]}`),
			},
		},
		parse: func(raw json.RawMessage) (any, error) {
			var dec alertDecision
			if err := json.Unmarshal(raw, &dec); err != nil {
				return nil, err
			}
			if strings.TrimSpace(dec.FireAt) == "" {
				return nil, fmt.Errorf("empty fire_at")
			}
			return alertProposedPayload{
				Message:    strings.TrimSpace(dec.Message),
				FireAt:     strings.TrimSpace(dec.FireAt),
				Recurrence: marshalRecurrence(dec.Recurrence),
			}, nil
		},
		valid: func(payload any, now time.Time) bool {
			p := payload.(alertProposedPayload)
			return p.Message != "" && futureOrRecurring(p.FireAt, p.Recurrence, now)
		},
		eventName:          "alert:proposed",
		successPlaceholder: "(propôs um alerta)",
		failurePlaceholder: "Não consegui agendar esse lembrete.",
	}
}

// newNoteProposalTool creates a proposalTool for the create_note function.
func newNoteProposalTool() *proposalTool {
	return &proposalTool{
		schema: ai.Tool{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "create_note",
				Description: "Cria uma anotação quando o usuário quer registrar uma informação importante (ideia, aprendizado, decisão etc.). Extraia um título curto, o conteúdo em Markdown e de 1 a 5 tags temáticas.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"title":{"type":"string","description":"Título curto da anotação"},"content":{"type":"string","description":"Conteúdo em Markdown"},"tags":{"type":"array","items":{"type":"string"},"description":"1 a 5 tags temáticas, minúsculas, sem #"}},"required":["title","content","tags"]}`),
			},
		},
		parse: func(raw json.RawMessage) (any, error) {
			var dec struct {
				Title   string   `json:"title"`
				Content string   `json:"content"`
				Tags    []string `json:"tags"`
			}
			if err := json.Unmarshal(raw, &dec); err != nil {
				return nil, err
			}
			title := strings.TrimSpace(dec.Title)
			content := strings.TrimSpace(dec.Content)
			if title == "" || content == "" {
				return nil, fmt.Errorf("empty title or content")
			}
			return noteProposedPayload{Title: title, Content: content, Tags: normalizeTags(dec.Tags)}, nil
		},
		valid: func(payload any, now time.Time) bool {
			p := payload.(noteProposedPayload)
			return p.Title != "" && p.Content != ""
		},
		eventName:          "note:proposed",
		successPlaceholder: "(propôs uma anotação)",
		failurePlaceholder: "Não consegui criar essa anotação — título ou conteúdo vazio.",
	}
}
