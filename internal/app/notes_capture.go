package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gix/internal/ai"
	"gix/internal/db"
	"gix/internal/embed"
)

// AlertProposal é um lembrete que o modelo de captura detectou dentro da nota.
// FireAt é ISO 8601 com offset (como o modelo retornou); Recurrence é JSON
// marshalado ou "". Presente só quando a nota tem horário/data concretos.
type AlertProposal struct {
	Message    string `json:"message"`
	FireAt     string `json:"fireAt"`
	Recurrence string `json:"recurrence"`
}

// CaptureResult is what the frontend gets after a /note.
//
//	"created"    note stored
//	"no_api_key" the API key is missing
//	"error"      failure (see Message)
type CaptureResult struct {
	Status    string         `json:"status"`
	NoteID    int64          `json:"noteId"`
	NoteTitle string         `json:"noteTitle"`
	Tags      []string       `json:"tags"`
	Message   string         `json:"message"`
	Tokens    int            `json:"tokens"`
	Cost      float64        `json:"cost"`
	Alert     *AlertProposal `json:"alert"`
}

// captureDecision is the JSON the model returns when formatting a capture.
type captureDecision struct {
	Title   string         `json:"title"`
	Content string         `json:"content"`
	Tags    []string       `json:"tags"`
	Alert   *alertDecision `json:"alert"`
}

// Capture formats a quick note with the AI (title + Markdown body + tags),
// stores it as one atomic note, and indexes it for full-text and (if the model
// is loaded) semantic search.
func (s *NotesService) Capture(text string) (CaptureResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return CaptureResult{Status: "error", Message: "empty"}, nil
	}
	if s.db == nil {
		return CaptureResult{Status: "error", Message: "no_db"}, nil
	}

	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return CaptureResult{Status: "no_api_key"}, nil
	}

	client := s.newClient(apiKey)
	raw, usage, err := client.Complete(context.Background(), cfg.Model, buildCapturePrompt(text, time.Now()))
	if err != nil {
		return CaptureResult{Status: "error", Message: err.Error()}, nil
	}
	dec, err := parseCaptureJSON(raw)
	if err != nil {
		return CaptureResult{Status: "error", Message: err.Error()}, nil
	}

	content := strings.TrimSpace(dec.Content)
	if content == "" {
		content = text
	}
	title := strings.TrimSpace(dec.Title)
	if title == "" {
		title = db.ExtractTitle(content)
	}
	tags := normalizeTags(dec.Tags)

	var vec []byte
	dim := 0
	if s.embedder != nil {
		if v, eerr := s.embedder.EmbedDoc(title + "\n" + content); eerr == nil {
			vec = embed.EncodeVector(v)
			dim = len(v)
		}
	}

	var proposal *AlertProposal
	if dec.Alert != nil {
		rec := marshalRecurrence(dec.Alert.Recurrence)
		if futureOrRecurring(dec.Alert.FireAt, rec, time.Now()) {
			msg := strings.TrimSpace(dec.Alert.Message)
			if msg == "" {
				msg = title
			}
			proposal = &AlertProposal{Message: msg, FireAt: strings.TrimSpace(dec.Alert.FireAt), Recurrence: rec}
		}
	}

	id, err := s.db.CreateNote(title, content, tags, vec, dim)
	if err != nil {
		return CaptureResult{}, err
	}
	tokens, cost := usageCost(usage, cfg.Model)
	return CaptureResult{Status: "created", NoteID: id, NoteTitle: title, Tags: tags, Tokens: tokens, Cost: cost, Alert: proposal}, nil
}

func parseCaptureJSON(s string) (captureDecision, error) {
	var dec captureDecision
	err := json.Unmarshal([]byte(stripFences(s)), &dec)
	return dec, err
}

func buildCapturePrompt(text string, now time.Time) []ai.Message {
	stamp, zoneName, offsetH := localTimeHeader(now)
	system := fmt.Sprintf(`Você organiza anotações rápidas do usuário em uma nota atômica e bem formatada.
A data e hora atuais são: %s. Fuso: %s (UTC%+d).
Resolva qualquer data relativa ("amanhã", "sexta") para uma data absoluta no texto.
Formate "content" como Markdown bem estruturado (parágrafo, lista, tarefa "- [ ]", ou pequena seção) — preserve a informação do usuário, sem inventar nem remover.
Gere um "title" curto (sem marcadores Markdown) e de 1 a 5 "tags" temáticas, minúsculas, sem "#".
Se o usuário pedir explicitamente para criar um alerta ou lembrete, extraia essa instrução para o campo "alert" — o conteúdo da nota deve conter apenas o restante do texto. Se não houver instrução explícita mas a nota descrever um lembrete com horário/data concretos, também inclua "alert". Caso contrário use "alert": null.
Responda APENAS com JSON, sem cercas:
{"title":"<título curto>","content":"<Markdown da nota>","tags":["tag1","tag2"],"alert":null ou {"message":"<lembrete curto>","fire_at":"<ISO 8601 com offset>","recurrence":null ou {"freq":"daily|weekly|monthly|yearly","interval":1,"weekday":"mon","time":"09:00"}}}`,
		stamp, zoneName, offsetH)
	return []ai.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: text},
	}
}
