package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gix/internal/ai"
	"gix/internal/config"
	"gix/internal/db"
)

type Emitter func(name string, data any)

type Streamer interface {
	StreamTools(ctx context.Context, model string, msgs []ai.Message, tools []ai.Tool, onDelta func(string)) (*ai.Usage, []ai.ToolCall, error)
}

type UsagePayload struct {
	Tokens int     `json:"tokens"`
	Cost   float64 `json:"cost"`
}

type DonePayload struct {
	Content string `json:"content"`
}

// createAlertTool deixa o modelo do chat agendar um lembrete em vez de só
// responder em prosa. O app intercepta a chamada e pede confirmação ao usuário.
var createAlertTool = ai.Tool{
	Type: "function",
	Function: ai.ToolFunction{
		Name:        "create_alert",
		Description: "Agenda um lembrete/alarme quando o usuário pede para ser lembrado de algo num horário ou data. Resolva datas relativas a partir do horário local atual informado no system prompt.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},"fire_at":{"type":"string","description":"ISO 8601 com offset"},"recurrence":{"type":["object","null"]}},"required":["message","fire_at"]}`),
	},
}

type alertProposedPayload struct {
	Message    string `json:"message"`
	FireAt     string `json:"fireAt"`
	Recurrence string `json:"recurrence"`
}

// chatToolSystem injeta o horário local atual para o modelo resolver datas
// relativas ao chamar create_alert.
func chatToolSystem(now time.Time, language string) ai.Message {
	zoneName, offsetSec := now.Zone()
	return ai.Message{Role: "system", Content: fmt.Sprintf(
		`Data e hora locais atuais: %s. Fuso: %s (UTC%+d). Idioma: %s. Se o usuário pedir um lembrete/alarme com horário ou data, chame a ferramenta create_alert (resolvendo datas relativas a ESTE momento) em vez de só responder.`,
		now.Format("2006-01-02 15:04 (Monday)"), zoneName, offsetSec/3600, language)}
}

func findToolCall(calls []ai.ToolCall, name string) (ai.ToolCall, bool) {
	for _, c := range calls {
		if c.Name == name {
			return c, true
		}
	}
	return ai.ToolCall{}, false
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

type ChatService struct {
	cfg       *ConfigService
	db        *db.Database
	emit      Emitter
	newClient func(apiKey string) Streamer

	mu         sync.Mutex
	convID     int64
	history    []ai.Message
	streaming  bool
	cancelFunc context.CancelFunc
	gen        uint64
	tokens     int
	cost       float64
}

func NewChatService(cfg *ConfigService, database *db.Database, emit Emitter, newClient func(apiKey string) Streamer) *ChatService {
	return &ChatService{cfg: cfg, db: database, emit: emit, newClient: newClient}
}

func (s *ChatService) NewConversation() {
	s.mu.Lock()
	s.convID = 0
	s.history = nil
	s.gen++
	s.tokens = 0
	s.cost = 0
	s.mu.Unlock()
}

func (s *ChatService) Cancel() {
	s.mu.Lock()
	cancel := s.cancelFunc
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *ChatService) Send(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		s.emit("chat:error", "no_api_key")
		return
	}

	s.mu.Lock()
	if s.streaming {
		s.mu.Unlock()
		return
	}
	s.streaming = true
	if s.convID == 0 && s.db != nil {
		if id, err := s.db.CreateConversation(db.ExtractTitle(text), cfg.Model); err == nil {
			s.convID = id
		}
	}
	cid := s.convID
	s.history = append(s.history, ai.Message{Role: "user", Content: text})
	msgs := make([]ai.Message, 0, len(s.history)+2)
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		msgs = append(msgs, ai.Message{Role: "system", Content: cfg.SystemPrompt})
	}
	msgs = append(msgs, chatToolSystem(time.Now(), cfg.Language))
	msgs = append(msgs, s.history...)
	gen := s.gen
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	s.mu.Unlock()

	if s.db != nil && cid != 0 {
		_ = s.db.AddMessage(cid, "user", text)
	}

	go func() {
		client := s.newClient(apiKey)
		var sb strings.Builder
		usage, toolCalls, streamErr := client.StreamTools(ctx, cfg.Model, msgs, []ai.Tool{createAlertTool}, func(delta string) {
			sb.WriteString(delta)
			s.emit("chat:delta", delta)
		})
		full := sb.String()

		s.mu.Lock()
		s.streaming = false
		s.cancelFunc = nil
		if usage != nil {
			s.tokens += usage.TotalTokens
			if p, ok := config.ModelPrices[cfg.Model]; ok {
				s.cost += p.CalculateCost(usage.PromptTokens, usage.CompletionTokens)
			}
		}
		tokens, cost := s.tokens, s.cost
		s.mu.Unlock()

		s.emit("chat:usage", UsagePayload{Tokens: tokens, Cost: cost})

		switch {
		case streamErr != nil && ctx.Err() == context.Canceled:
			if full != "" {
				s.persist(cid, gen, full)
			}
		case streamErr != nil:
			s.emit("chat:error", streamErr.Error())
		default:
			if call, ok := findToolCall(toolCalls, "create_alert"); ok {
				if p, perr := parseAlertCall(call); perr == nil {
					if p.Message != "" && futureOrRecurring(p.FireAt, p.Recurrence, time.Now()) {
						s.emit("alert:proposed", p)
						if full != "" {
							s.persist(cid, gen, full)
							s.emit("chat:done", DonePayload{Content: full})
						} else {
							s.persist(cid, gen, "(propôs um alerta)")
						}
						return
					}
					// Tool call that can't be scheduled (past time / empty message):
					// give the user feedback instead of a dead-end chip.
					if full == "" {
						full = "Não consegui agendar esse lembrete."
					}
					s.persist(cid, gen, full)
					s.emit("chat:done", DonePayload{Content: full})
					return
				}
			}
			if full == "" {
				full = "(sem resposta)"
			}
			s.persist(cid, gen, full)
			s.emit("chat:done", DonePayload{Content: full})
		}
	}()
}

func (s *ChatService) persist(cid int64, gen uint64, full string) {
	if s.db != nil && cid != 0 {
		_ = s.db.AddMessage(cid, "assistant", full)
	}
	s.mu.Lock()
	if s.gen == gen {
		s.history = append(s.history, ai.Message{Role: "assistant", Content: full})
	}
	s.mu.Unlock()
}
