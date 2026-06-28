package app

import (
	"fmt"
	"strings"

	"gix/internal/db"
	"gix/internal/embed"
)

// maxCandidates is how many existing notes (most semantically similar first) are
// offered to the model when deciding whether a capture should attach instead of
// creating a new note. Small on purpose: enough to find the right home, few
// enough to keep the prompt cheap.
const maxCandidates = 5

// AttachProposal points a capture at an existing note the model judged it
// belongs to. Capture returns it (status "attach_proposed") instead of writing,
// so the frontend can confirm before AppendTo runs.
type AttachProposal struct {
	TargetID    int64  `json:"targetId"`
	TargetTitle string `json:"targetTitle"`
}

// candidateNotes returns the notes most semantically similar to text, best-first,
// for the capture router. Purely semantic (vector) so an arbitrary note body
// can't trip FTS query syntax; returns nil when the embedder isn't ready yet, in
// which case capture always creates.
func (s *NotesService) candidateNotes(text string) []db.Note {
	if s.db == nil {
		return nil
	}
	ids, err := s.vectorSearch(text)
	if err != nil || len(ids) == 0 {
		return nil
	}
	if len(ids) > maxCandidates {
		ids = ids[:maxCandidates]
	}
	notes, err := s.db.NotesByIDs(ids)
	if err != nil {
		return nil
	}
	byID := make(map[int64]db.Note, len(notes))
	for _, n := range notes {
		byID[n.ID] = n
	}
	out := make([]db.Note, 0, len(ids)) // preserve the best-first order of ids
	for _, id := range ids {
		if n, ok := byID[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

// validAttach resolves the model's attach_to id against the candidates it was
// actually shown, guarding against a hallucinated id. Returns the target note
// and true only on a real match.
func validAttach(attachTo *int64, cands []db.Note) (db.Note, bool) {
	if attachTo == nil {
		return db.Note{}, false
	}
	for _, n := range cands {
		if n.ID == *attachTo {
			return n, true
		}
	}
	return db.Note{}, false
}

// candidatesBlock renders the candidate notes for the capture prompt, with the
// rule for when to set attach_to. Empty when there are no candidates, so the
// prompt offers no attach option and the model leaves attach_to null.
func candidatesBlock(cands []db.Note) string {
	if len(cands) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nAnotações já existentes possivelmente relacionadas:\n")
	for _, n := range cands {
		fmt.Fprintf(&b, "- id %d — %s: %s\n", n.ID, n.Title, snippet(n.Content))
	}
	b.WriteString(`Se este novo texto for claramente a MESMA anotação/assunto de UMA delas (complementando-a, não apenas um tema parecido), coloque o id dela em "attach_to" para anexar. Na dúvida, ou se for algo novo, use "attach_to": null.`)
	return b.String()
}

// AppendTo appends already-formatted content to the end of an existing note,
// re-embedding the combined text and keeping FTS/vector/tags in sync (tags are
// the union of old and new). Used when the capture router decides to attach and
// the user confirms. No AI call.
func (s *NotesService) AppendTo(targetID int64, content string, tags []string) (CaptureResult, error) {
	if s.db == nil {
		return CaptureResult{Status: "error", Message: "no_db"}, nil
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return CaptureResult{Status: "error", Message: "empty"}, nil
	}
	note, err := s.db.GetNote(targetID)
	if err != nil {
		return CaptureResult{Status: "error", Message: err.Error()}, nil
	}

	merged := strings.TrimSpace(note.Content) + "\n\n" + content
	mergedTags := normalizeTagsUncapped(append(append([]string{}, note.Tags...), tags...))

	var vec []byte
	dim := 0
	if s.embedder != nil {
		if v, eerr := s.embedder.EmbedDoc(note.Title + "\n" + merged); eerr == nil {
			vec = embed.EncodeVector(v)
			dim = len(v)
		}
	}
	if err := s.db.UpdateNote(targetID, note.Title, merged, mergedTags, vec, dim); err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{Status: "attached", NoteID: targetID, NoteTitle: note.Title, Tags: mergedTags}, nil
}
