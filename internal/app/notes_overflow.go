package app

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"gix/internal/ai"
	"gix/internal/db"
)

// OverflowProposal is returned (status "overflow_proposed") when appending to a
// note would push it past its size limit. The frontend asks the user which
// strategy to take and calls ResolveOverflow; nothing is written until then.
type OverflowProposal struct {
	TargetID    int64  `json:"targetId"`
	TargetTitle string `json:"targetTitle"`
	Length      int    `json:"length"` // projected character count after the append
	Limit       int    `json:"limit"`  // the effective limit it would exceed
}

// effectiveLimit is the note's own char limit when set (>0), otherwise the global
// default from config. A non-positive result means "no limit" (overflow off).
func (s *NotesService) effectiveLimit(note db.Note) int {
	if note.CharLimit > 0 {
		return note.CharLimit
	}
	return s.cfg.Current().NoteCharLimit
}

// overflowProposal returns a proposal when merged exceeds the note's effective
// limit, or nil when it fits (or limits are disabled).
func (s *NotesService) overflowProposal(note db.Note, merged string) *OverflowProposal {
	limit := s.effectiveLimit(note)
	if limit <= 0 {
		return nil
	}
	n := utf8.RuneCountInString(merged)
	if n <= limit {
		return nil
	}
	return &OverflowProposal{TargetID: note.ID, TargetTitle: note.Title, Length: n, Limit: limit}
}

// ResolveOverflow applies the user's chosen strategy for content that overflowed
// a note (see AppendTo). mode is one of:
//
//	"summarize" — merge old+new, ask the AI to condense the combined note to fit
//	"part2"     — leave the note as-is, put the new content in a linked sibling
//	"split"     — merge, then ask the AI to divide everything into themed notes
func (s *NotesService) ResolveOverflow(targetID int64, content string, tags []string, mode string) (CaptureResult, error) {
	if s.db == nil {
		return CaptureResult{Status: "error", Message: "no_db"}, nil
	}
	content = strings.TrimSpace(content)
	note, err := s.db.GetNote(targetID)
	if err != nil {
		return CaptureResult{Status: "error", Message: err.Error()}, nil
	}
	switch mode {
	case "part2":
		return s.overflowPart2(note.Title, content, unionTags(note.Tags, tags))
	case "summarize":
		return s.overflowSummarize(note, content, tags)
	case "split":
		return s.overflowSplit(note, content)
	default:
		return CaptureResult{Status: "error", Message: "unknown_mode"}, nil
	}
}

// overflowPart2 keeps the original note untouched and stores the overflowing
// content in a new sibling note, titled "<title> · parte N" (N incremented when
// the title already ends in one). No AI call.
func (s *NotesService) overflowPart2(title, content string, tags []string) (CaptureResult, error) {
	return s.CreateFromProposal(nextPartTitle(title), content, tags)
}

// overflowSummarize merges the new content into the note and replaces the body
// with an AI summary of the whole, so the combined note fits.
func (s *NotesService) overflowSummarize(note db.Note, content string, tags []string) (CaptureResult, error) {
	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return CaptureResult{Status: "no_api_key"}, nil
	}
	merged := strings.TrimSpace(note.Content) + "\n\n" + content
	client := s.newClient(apiKey)
	raw, usage, err := client.Complete(context.Background(), cfg.Model,
		buildNoteSummaryPrompt(db.Note{Title: note.Title, Content: merged}, cfg.Language))
	if err != nil {
		return CaptureResult{Status: "error", Message: err.Error()}, nil
	}
	summary := strings.TrimSpace(stripFences(raw))
	if summary == "" {
		summary = merged
	}
	mergedTags := unionTags(note.Tags, tags)
	if err := s.storeNoteBody(note.ID, note.Title, summary, mergedTags); err != nil {
		return CaptureResult{}, err
	}
	tokens, cost := usageCost(usage, cfg.Model)
	return CaptureResult{Status: "attached", NoteID: note.ID, NoteTitle: note.Title, Tags: mergedTags, Tokens: tokens, Cost: cost}, nil
}

// splitDecision is the JSON the model returns when dividing a note by theme.
type splitDecision struct {
	Notes []struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	} `json:"notes"`
}

// overflowSplit merges the new content in, asks the AI to divide everything into
// themed notes (preserving every fact), then replaces the original with the new
// set. Falls back to a plain append if the model returns nothing usable.
func (s *NotesService) overflowSplit(note db.Note, content string) (CaptureResult, error) {
	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return CaptureResult{Status: "no_api_key"}, nil
	}
	merged := strings.TrimSpace(note.Content) + "\n\n" + content
	client := s.newClient(apiKey)
	raw, usage, err := client.Complete(context.Background(), cfg.Model, buildNoteSplitPrompt(note.Title, merged, cfg.Language))
	if err != nil {
		return CaptureResult{Status: "error", Message: err.Error()}, nil
	}
	tokens, cost := usageCost(usage, cfg.Model)

	var dec splitDecision
	if jerr := json.Unmarshal([]byte(stripFences(raw)), &dec); jerr != nil || len(dec.Notes) < 2 {
		// Not a usable split — keep everything in the original note rather than lose data.
		if err := s.storeNoteBody(note.ID, note.Title, merged, note.Tags); err != nil {
			return CaptureResult{}, err
		}
		return CaptureResult{Status: "attached", NoteID: note.ID, NoteTitle: note.Title, Tags: note.Tags, Tokens: tokens, Cost: cost}, nil
	}

	// Create every part first; only drop the original once they're all stored, so
	// a mid-way failure never loses the source note.
	var firstID int64
	created := 0
	for _, p := range dec.Notes {
		title := strings.TrimSpace(p.Title)
		body := strings.TrimSpace(p.Content)
		if body == "" {
			continue
		}
		if title == "" {
			title = db.ExtractTitle(body)
		}
		vec, dim := s.embedFor(title, body)
		id, err := s.db.CreateNote(title, body, normalizeTags(p.Tags), vec, dim)
		if err != nil {
			return CaptureResult{}, err
		}
		if firstID == 0 {
			firstID = id
		}
		created++
	}
	if err := s.db.DeleteNote(note.ID); err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{Status: "split", NoteID: firstID, NoteTitle: note.Title, Count: created, Tokens: tokens, Cost: cost}, nil
}

func buildNoteSplitPrompt(title, content, language string) []ai.Message {
	system := fmt.Sprintf(`Você divide uma anotação longa em VÁRIAS anotações menores, agrupadas por tema/assunto.
Preserve TODA a informação e os fatos — não invente, não remova e não resuma. Cada informação deve aparecer em exatamente uma das notas.
Para cada nota gere um "title" curto e específico (3 a 6 palavras, sem marcadores Markdown) e de 1 a 5 "tags" temáticas, minúsculas, sem "#". Formate "content" como Markdown bem estruturado.
Crie de 2 a 6 notas. Idioma da resposta: %s. Responda APENAS com JSON, sem cercas:
{"notes":[{"title":"<título>","content":"<Markdown>","tags":["tag1"]}]}`, language)
	user := fmt.Sprintf("%s\n\n%s", title, content)
	return []ai.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
}

// partTitleRe matches a title already ending in "· parte N" so the next sibling
// continues the numbering instead of stacking "· parte 2 · parte 2".
var partTitleRe = regexp.MustCompile(`^(.*?)\s*·\s*parte\s+(\d+)$`)

func nextPartTitle(title string) string {
	title = strings.TrimSpace(title)
	if m := partTitleRe.FindStringSubmatch(title); m != nil {
		n, _ := strconv.Atoi(m[2])
		return fmt.Sprintf("%s · parte %d", strings.TrimSpace(m[1]), n+1)
	}
	return title + " · parte 2"
}

// unionTags merges two tag sets (normalized, uncapped, de-duped) preserving order.
func unionTags(a, b []string) []string {
	return normalizeTagsUncapped(append(append([]string{}, a...), b...))
}
