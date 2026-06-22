package app

import (
	"context"
	"strings"
	"sync"

	"gix/internal/ai"
	"gix/internal/config"
	"gix/internal/db"
)

type Emitter func(name string, data any)

type Streamer interface {
	Stream(ctx context.Context, model string, msgs []ai.Message, onDelta func(string)) (*ai.Usage, error)
}

type UsagePayload struct {
	Tokens int     `json:"tokens"`
	Cost   float64 `json:"cost"`
}

type DonePayload struct {
	Content string `json:"content"`
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
	msgs := make([]ai.Message, 0, len(s.history)+1)
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		msgs = append(msgs, ai.Message{Role: "system", Content: cfg.SystemPrompt})
	}
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
		usage, streamErr := client.Stream(ctx, cfg.Model, msgs, func(delta string) {
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
