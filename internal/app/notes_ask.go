package app

import (
	"context"
	"fmt"
	"strings"

	"gix/internal/ai"
	"gix/internal/db"
)

const askTopK = 6 // notes fed to the AI for /ask

// AskResult is /find plus an AI summary of the top notes.
type AskResult struct {
	Status  string         `json:"status"`
	Summary string         `json:"summary"`
	Sources []SearchResult `json:"sources"`
	Message string         `json:"message"`
	Tokens  int            `json:"tokens"`
	Cost    float64        `json:"cost"`
}

// Ask searches, then asks the AI to summarize the top notes in answer to the
// query. Returns the summary plus the source notes it drew from.
func (s *NotesService) Ask(query string) (AskResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return AskResult{Status: "error", Message: "empty"}, nil
	}

	results, err := s.Find(query)
	if err != nil {
		return AskResult{}, err
	}
	if len(results) == 0 {
		return AskResult{Status: "empty", Sources: nil}, nil
	}
	if len(results) > askTopK {
		results = results[:askTopK]
	}

	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return AskResult{Status: "no_api_key", Sources: results}, nil
	}

	client := s.newClient(apiKey)
	raw, usage, err := client.Complete(context.Background(), cfg.Model, buildSummarizePrompt(query, results))
	if err != nil {
		return AskResult{Status: "error", Message: err.Error(), Sources: results}, nil
	}
	tokens, cost := usageCost(usage, cfg.Model)
	return AskResult{
		Status:  "ok",
		Summary: strings.TrimSpace(stripFences(raw)),
		Sources: results,
		Tokens:  tokens,
		Cost:    cost,
	}, nil
}

// SummarizeResult is the AI summary of a single note. The summary is returned
// only; applying it (replacing the note body) is done by the caller via Update,
// which keeps undo symmetric across the command and the notes-view button.
//
//	"ok"         summary produced
//	"no_api_key" the API key is missing
//	"empty"      the note has no content to summarize
//	"error"      failure (see Message)
type SummarizeResult struct {
	Status  string  `json:"status"`
	Summary string  `json:"summary"`
	Message string  `json:"message"`
	Tokens  int     `json:"tokens"`
	Cost    float64 `json:"cost"`
}

// Summarize asks the AI to condense one note into a shorter Markdown summary. It
// does not modify the note; the frontend applies the result via Update (so the
// change is undoable) when the user confirms.
func (s *NotesService) Summarize(id int64) (SummarizeResult, error) {
	if s.db == nil {
		return SummarizeResult{Status: "error", Message: "no_db"}, nil
	}
	note, err := s.db.GetNote(id)
	if err != nil {
		return SummarizeResult{Status: "error", Message: err.Error()}, nil
	}
	if strings.TrimSpace(note.Content) == "" {
		return SummarizeResult{Status: "empty"}, nil
	}

	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return SummarizeResult{Status: "no_api_key"}, nil
	}

	client := s.newClient(apiKey)
	raw, usage, err := client.Complete(context.Background(), cfg.Model, buildNoteSummaryPrompt(note, cfg.Language))
	if err != nil {
		return SummarizeResult{Status: "error", Message: err.Error()}, nil
	}
	tokens, cost := usageCost(usage, cfg.Model)
	return SummarizeResult{
		Status:  "ok",
		Summary: strings.TrimSpace(stripFences(raw)),
		Tokens:  tokens,
		Cost:    cost,
	}, nil
}

func buildNoteSummaryPrompt(note db.Note, language string) []ai.Message {
	system := fmt.Sprintf(`Você resume uma anotação do usuário de forma concisa em Markdown.
Preserve os pontos principais; não invente nem adicione informação que não esteja na nota.
Mantenha o resultado bem mais curto que o original, em estrutura clara (parágrafos curtos, listas ou tarefas "- [ ]" quando fizer sentido).
Idioma da resposta: %s. Responda APENAS com o resumo, sem preâmbulo nem cercas.`, language)
	user := fmt.Sprintf("%s\n\n%s", note.Title, note.Content)
	return []ai.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
}

func buildSummarizePrompt(query string, results []SearchResult) []ai.Message {
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "[%d] %s\n%s\n---\n", i+1, r.Title, r.Content)
	}
	system := `Você responde à pergunta do usuário usando APENAS as anotações fornecidas abaixo.
Resuma de forma direta em Markdown. Não invente informação que não esteja nas notas.
Se as notas não responderem à pergunta, diga isso claramente.`
	user := fmt.Sprintf("Pergunta:\n%s\n\nAnotações:\n%s", query, b.String())
	return []ai.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
}
