package app

import (
	"context"
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

// baseSystemPrompt define a identidade do assistente e as regras de uso das
// ferramentas disponíveis (create_note, create_alert). Fica separado do prompt
// editável pelo usuário (cfg.SystemPrompt) — ambos são enviados à IA.
func baseSystemPrompt() string {
	return `Você é um assistente pessoal rápido e versátil, similar ao Spotlight/Raycast com IA.
Seu propósito PRINCIPAL é responder perguntas, fazer pesquisas rápidas, ajudar com dúvidas e fornecer informações.

Você tem duas ferramentas auxiliares:

[create_note] — USE quando:
- O usuário disser explicitamente "anota isso", "registra", "guarda aí", "salva"
- O usuário compartilhar uma ideia, aprendizado, decisão ou insight importante
- O usuário descrever algo que merece ser registrado para referência futura
- O usuário mencionar configurações, comandos, dicas, receitas ou listas úteis
- Durante uma conversa, surgir informação que claramente o usuário vai querer consultar depois
NÃO use create_note para perguntas comuns ou conversa casual.

[create_alert] — USE quando:
- O usuário pedir "lembrete", "alarme", "alerta", "me lembre", "despertador"
- O usuário mencionar algo para fazer em um horário ou data específica
- O usuário usar expressões como "amanhã", "daqui a X horas", "às X horas", "segunda", "próxima semana"
- O usuário descrever um compromisso, tarefa com prazo ou evento futuro
NÃO use create_alert se não houver menção a tempo/horário.

REGRAS IMPORTANTES:
- Responda perguntas primeiro — as ferramentas são complementares
- Não invente informações que não conhece
- Se não souber a resposta, diga que não sabe
- Se o usuário pedir algo que não está ao seu alcance, explique educadamente`
}

// chatToolSystem injeta o horário local atual para o modelo resolver datas
// relativas ao chamar create_alert.
func chatToolSystem(now time.Time, language string) ai.Message {
	stamp, zoneName, offsetH := localTimeHeader(now)
	return ai.Message{Role: "system", Content: fmt.Sprintf(
		`Data e hora locais atuais: %s. Fuso: %s (UTC%+d). Idioma: %s.`,
		stamp, zoneName, offsetH, language)}
}

type ChatService struct {
	cfg       *ConfigService
	db        *db.Database
	emit      Emitter
	newClient func(apiKey string) Streamer
	tools     toolRegistry

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
	return &ChatService{cfg: cfg, db: database, emit: emit, newClient: newClient, tools: defaultChatTools()}
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
	msgs := make([]ai.Message, 0, len(s.history)+3)
	msgs = append(msgs, ai.Message{Role: "system", Content: baseSystemPrompt()})
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
		usage, toolCalls, streamErr := client.StreamTools(ctx, cfg.Model, msgs, s.tools.schemas(), func(delta string) {
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
			result := s.tools.dispatch(toolCalls, s.emit, time.Now())
			text := full
			if text == "" {
				text = result.Placeholder
				if text == "" {
					text = "(sem resposta)"
				}
			}
			s.persist(cid, gen, text)
			if full != "" || !result.SuppressDone {
				s.emit("chat:done", DonePayload{Content: text})
			}
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
