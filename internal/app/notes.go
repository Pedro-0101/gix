package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gix/internal/ai"
	"gix/internal/config"
	"gix/internal/db"
)

// Completer é a parte do cliente de IA que o NotesService precisa: uma chamada
// não-streaming que devolve a resposta inteira (JSON). Injetada para testes.
type Completer interface {
	Complete(ctx context.Context, model string, msgs []ai.Message) (string, *ai.Usage, error)
}

type NotesService struct {
	cfg       *ConfigService
	db        *db.Database
	newClient func(apiKey string) Completer
}

func NewNotesService(cfg *ConfigService, database *db.Database, newClient func(apiKey string) Completer) *NotesService {
	return &NotesService{cfg: cfg, db: database, newClient: newClient}
}

// RouteResult é o que o frontend recebe após um /note. Status:
//   - "created"    nova nota criada
//   - "appended"   item anexado a uma nota existente
//   - "full"       a nota alvo estouraria o limite — pede escolha ao usuário
//   - "no_api_key" falta a chave da API
//   - "error"      falha (mensagem em Message)
type RouteResult struct {
	Status    string  `json:"status"`
	NoteTitle string  `json:"noteTitle"`
	NoteID    int64   `json:"noteId"`
	Message   string  `json:"message"`
	Tokens    int     `json:"tokens"`
	Cost      float64 `json:"cost"`
}

// routeDecision é o JSON que o modelo devolve no roteamento.
type routeDecision struct {
	Action        string `json:"action"`
	NoteID        int64  `json:"note_id"`
	Title         string `json:"title"`
	FormattedItem string `json:"formatted_item"`
}

// Route decide o destino do texto e aplica a anotação. Ver RouteResult.
func (s *NotesService) Route(text string) (RouteResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return RouteResult{Status: "error", Message: "empty"}, nil
	}
	if s.db == nil {
		return RouteResult{Status: "error", Message: "no_db"}, nil
	}

	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return RouteResult{Status: "no_api_key"}, nil
	}

	notes, err := s.db.ListNotes()
	if err != nil {
		return RouteResult{}, err
	}

	client := s.newClient(apiKey)
	raw, usage, err := client.Complete(context.Background(), cfg.Model, buildRoutePrompt(text, notes, time.Now()))
	if err != nil {
		return RouteResult{Status: "error", Message: err.Error()}, nil
	}
	dec, err := parseRouteJSON(raw)
	if err != nil {
		return RouteResult{Status: "error", Message: err.Error()}, nil
	}

	tokens, cost := usageCost(usage, cfg.Model)

	if dec.Action == "create" || dec.NoteID == 0 {
		title := strings.TrimSpace(dec.Title)
		if title == "" {
			title = db.ExtractTitle(dec.FormattedItem)
		}
		id, err := s.db.CreateNote(title, dec.FormattedItem, 0, "")
		if err != nil {
			return RouteResult{}, err
		}
		return RouteResult{Status: "created", NoteTitle: title, NoteID: id, Tokens: tokens, Cost: cost}, nil
	}

	note, err := s.db.GetNote(dec.NoteID)
	if err != nil {
		// O modelo apontou para uma nota inexistente: cria uma nova como fallback.
		title := strings.TrimSpace(dec.Title)
		if title == "" {
			title = db.ExtractTitle(dec.FormattedItem)
		}
		id, cerr := s.db.CreateNote(title, dec.FormattedItem, 0, "")
		if cerr != nil {
			return RouteResult{}, cerr
		}
		return RouteResult{Status: "created", NoteTitle: title, NoteID: id, Tokens: tokens, Cost: cost}, nil
	}

	mode := effectiveMode(note.IntegrationMode, cfg.NotesIntegrationMode)
	var prospective string
	if mode == "rewrite" {
		rewritten, u2, rerr := client.Complete(context.Background(), cfg.Model, buildRewritePrompt(note, dec.FormattedItem))
		if rerr != nil {
			return RouteResult{Status: "error", Message: rerr.Error()}, nil
		}
		t2, c2 := usageCost(u2, cfg.Model)
		tokens += t2
		cost += c2
		prospective = strings.TrimSpace(rewritten)
	} else {
		prospective = appendLine(note.Content, dec.FormattedItem)
	}

	limit := effectiveLimit(note.LineLimit, cfg.NotesLineLimit)
	if exceedsLimit(prospective, limit) {
		return RouteResult{Status: "full", NoteTitle: note.Title, NoteID: note.ID, Tokens: tokens, Cost: cost}, nil
	}
	if err := s.db.UpdateNoteContent(note.ID, prospective); err != nil {
		return RouteResult{}, err
	}
	return RouteResult{Status: "appended", NoteTitle: note.Title, NoteID: note.ID, Tokens: tokens, Cost: cost}, nil
}

// ResolveOverflow executa a escolha do usuário quando uma nota estoura o limite.
// strategy: "summarize" | "part2" | "split".
func (s *NotesService) ResolveOverflow(noteID int64, text, strategy string) (RouteResult, error) {
	if s.db == nil {
		return RouteResult{Status: "error", Message: "no_db"}, nil
	}
	note, err := s.db.GetNote(noteID)
	if err != nil {
		return RouteResult{}, err
	}
	cfg := s.cfg.Current()

	switch strategy {
	case "part2":
		// Sem IA: cria uma nota irmã com o texto cru.
		title := note.Title + " 2"
		id, cerr := s.db.CreateNote(title, strings.TrimSpace(text), note.LineLimit, note.IntegrationMode)
		if cerr != nil {
			return RouteResult{}, cerr
		}
		return RouteResult{Status: "created", NoteTitle: title, NoteID: id}, nil

	case "summarize":
		apiKey := cfg.ResolveAPIKey()
		if apiKey == "" {
			return RouteResult{Status: "no_api_key"}, nil
		}
		client := s.newClient(apiKey)
		raw, usage, cerr := client.Complete(context.Background(), cfg.Model, buildSummarizePrompt(note, text))
		if cerr != nil {
			return RouteResult{Status: "error", Message: cerr.Error()}, nil
		}
		tokens, cost := usageCost(usage, cfg.Model)
		if err := s.db.UpdateNoteContent(note.ID, strings.TrimSpace(raw)); err != nil {
			return RouteResult{}, err
		}
		return RouteResult{Status: "appended", NoteTitle: note.Title, NoteID: note.ID, Tokens: tokens, Cost: cost}, nil

	case "split":
		apiKey := cfg.ResolveAPIKey()
		if apiKey == "" {
			return RouteResult{Status: "no_api_key"}, nil
		}
		client := s.newClient(apiKey)
		raw, usage, cerr := client.Complete(context.Background(), cfg.Model, buildSplitPrompt(note, text))
		if cerr != nil {
			return RouteResult{Status: "error", Message: cerr.Error()}, nil
		}
		parts, perr := parseSplitJSON(raw)
		if perr != nil || len(parts) == 0 {
			return RouteResult{Status: "error", Message: "split_parse"}, nil
		}
		tokens, cost := usageCost(usage, cfg.Model)
		if err := s.db.DeleteNote(note.ID); err != nil {
			return RouteResult{}, err
		}
		var firstTitle string
		for i, p := range parts {
			id, cerr := s.db.CreateNote(p.Title, p.Content, note.LineLimit, note.IntegrationMode)
			if cerr != nil {
				return RouteResult{}, cerr
			}
			if i == 0 {
				firstTitle = p.Title
				_ = id
			}
		}
		return RouteResult{Status: "split", NoteTitle: firstTitle, Tokens: tokens, Cost: cost}, nil

	default:
		return RouteResult{Status: "error", Message: "unknown_strategy"}, nil
	}
}

type splitPart struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// --- helpers puros ---

// stripFences remove cercas de bloco ```json … ``` ao redor de uma resposta.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i != -1 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

func parseRouteJSON(s string) (routeDecision, error) {
	var dec routeDecision
	err := json.Unmarshal([]byte(stripFences(s)), &dec)
	return dec, err
}

func parseSplitJSON(s string) ([]splitPart, error) {
	var parts []splitPart
	err := json.Unmarshal([]byte(stripFences(s)), &parts)
	return parts, err
}

// appendLine cola item ao fim do conteúdo, garantindo uma quebra de linha.
func appendLine(content, item string) string {
	content = strings.TrimRight(content, "\n")
	item = strings.TrimSpace(item)
	if content == "" {
		return item
	}
	return content + "\n" + item
}

// exceedsLimit informa se o conteúdo passa do limite de linhas. limit 0 = ilimitado.
func exceedsLimit(content string, limit int) bool {
	if limit <= 0 {
		return false
	}
	return countLines(content) > limit
}

func countLines(content string) int {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func effectiveLimit(noteLimit, globalLimit int) int {
	if noteLimit > 0 {
		return noteLimit
	}
	return globalLimit
}

func effectiveMode(noteMode, globalMode string) string {
	if noteMode != "" {
		return noteMode
	}
	return globalMode
}

func usageCost(usage *ai.Usage, model string) (int, float64) {
	if usage == nil {
		return 0, 0
	}
	cost := 0.0
	if p, ok := config.ModelPrices[model]; ok {
		cost = p.CalculateCost(usage.PromptTokens, usage.CompletionTokens)
	}
	return usage.TotalTokens, cost
}

// --- prompts ---

func notesContext(notes []db.Note) string {
	if len(notes) == 0 {
		return "(nenhuma nota existente)"
	}
	var b strings.Builder
	for _, n := range notes {
		fmt.Fprintf(&b, "id=%d | título=%q | modo=%s\n%s\n---\n", n.ID, n.Title, n.IntegrationMode, n.Content)
	}
	return b.String()
}

func buildRoutePrompt(text string, notes []db.Note, now time.Time) []ai.Message {
	system := fmt.Sprintf(`Você roteia anotações rápidas do usuário para notas.
A data e hora atuais são: %s.
Decida se a anotação deve ser ANEXADA a uma nota existente de tema compatível ou se uma NOVA nota deve ser criada.
Resolva qualquer data relativa ("amanhã", "sexta") para uma data absoluta no próprio texto formatado.
Responda APENAS com JSON, sem cercas, neste formato:
{"action":"append"|"create","note_id":<id da nota se append, senão 0>,"title":<título curto se create, senão "">,"formatted_item":"<a anotação como UMA linha, formatada e com a data já absoluta>"}

Notas existentes:
%s`, now.Format("2006-01-02 15:04 (Monday)"), notesContext(notes))
	return []ai.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: text},
	}
}

func buildRewritePrompt(note db.Note, item string) []ai.Message {
	system := `Você reorganiza uma nota incorporando um novo item.
Agrupe itens semelhantes e normalize o formato, mas NÃO invente nem remova informação do usuário.
Responda APENAS com o texto completo e atualizado da nota, sem comentários nem cercas.`
	user := fmt.Sprintf("Nota atual:\n%s\n\nNovo item a incorporar:\n%s", note.Content, item)
	return []ai.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
}

func buildSummarizePrompt(note db.Note, item string) []ai.Message {
	system := `A nota está cheia. Condense-a preservando a informação essencial e então inclua o novo item.
Responda APENAS com o texto completo e resumido da nota, sem comentários nem cercas.`
	user := fmt.Sprintf("Nota atual:\n%s\n\nNovo item a incluir:\n%s", note.Content, item)
	return []ai.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
}

func buildSplitPrompt(note db.Note, item string) []ai.Message {
	system := `A nota mistura temas diferentes e está cheia. Divida-a (incluindo o novo item) em 2 ou mais notas temáticas.
Responda APENAS com JSON, sem cercas, um array: [{"title":"...","content":"..."}]`
	user := fmt.Sprintf("Nota atual (título %q):\n%s\n\nNovo item:\n%s", note.Title, note.Content, item)
	return []ai.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
}
