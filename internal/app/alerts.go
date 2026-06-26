package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"gix/internal/ai"
	"gix/internal/db"
)

// pollInterval is how often the scheduler scans for due alerts.
const pollInterval = 30 * time.Second

// alertCategoryID groups the toast action buttons (snooze / done).
const alertCategoryID = "gix.alert"

// notifier is the slice of the Wails NotificationService the scheduler needs.
// Injected so tests can fake it (and so a nil notifier degrades to overlay-only).
type notifier interface {
	SendNotificationWithActions(options notifications.NotificationOptions) error
	RegisterNotificationCategory(category notifications.NotificationCategory) error
}

// AlertsService creates reminders (AI-parsed) and fires them on a polling loop.
type AlertsService struct {
	cfg       *ConfigService
	db        *db.Database
	newClient func(apiKey string) Completer
	emit      Emitter
	onShow    func()
	notifier  notifier
	loc       *time.Location // recurrence/display location; time.Local in prod
}

func NewAlertsService(cfg *ConfigService, database *db.Database, newClient func(apiKey string) Completer, emit Emitter, onShow func(), n notifier) *AlertsService {
	return &AlertsService{cfg: cfg, db: database, newClient: newClient, emit: emit, onShow: onShow, notifier: n, loc: time.Local}
}

// CreateAlertResult is what the frontend gets after /alerta or "create from note".
//
//	"created"     alert stored
//	"no_api_key"  the API key is missing
//	"unparseable" the AI couldn't produce valid JSON / a valid time
//	"past"        a one-shot time already in the past
//	"error"       failure (e.g. DB)
type CreateAlertResult struct {
	Status      string `json:"status"`
	AlertID     int64  `json:"alertId"`
	Message     string `json:"message"`
	FireAtLocal string `json:"fireAtLocal"`
	Recurrence  string `json:"recurrence"`
}

// alertDecision is the JSON the model returns for an alert request.
type alertDecision struct {
	Message    string          `json:"message"`
	FireAt     string          `json:"fire_at"`    // ISO 8601 with offset
	Recurrence *recurrenceRule `json:"recurrence"` // null = one-shot
}

// Create turns a natural-language reminder into a stored alert (1 AI call).
func (s *AlertsService) Create(text string) (CreateAlertResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return CreateAlertResult{Status: "error"}, nil
	}
	return s.createFromWhen(text, "", nil)
}

// CreateForNote creates an alert linked to a note: the AI parses only the timing
// and the message defaults to the note's title.
func (s *AlertsService) CreateForNote(noteID int64, whenText string) (CreateAlertResult, error) {
	if s.db == nil {
		return CreateAlertResult{Status: "error"}, nil
	}
	note, err := s.db.GetNote(noteID)
	if err != nil {
		return CreateAlertResult{Status: "error"}, nil
	}
	id := noteID
	return s.createFromWhen(whenText, note.Title, &id)
}

// createFromWhen is the shared path: parse timing with the AI, validate, store.
// defaultMessage (note title) is used when the AI returns an empty message.
func (s *AlertsService) createFromWhen(text, defaultMessage string, noteID *int64) (CreateAlertResult, error) {
	if s.db == nil {
		return CreateAlertResult{Status: "error"}, nil
	}
	cfg := s.cfg.Current()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return CreateAlertResult{Status: "no_api_key"}, nil
	}

	dec, err := s.parseWhen(text, defaultMessage, time.Now())
	if err != nil {
		return CreateAlertResult{Status: "unparseable"}, nil
	}

	fireAt, err := time.Parse(time.RFC3339, strings.TrimSpace(dec.FireAt))
	if err != nil {
		return CreateAlertResult{Status: "unparseable"}, nil
	}
	fireAt = fireAt.UTC()

	message := strings.TrimSpace(dec.Message)
	if message == "" {
		message = strings.TrimSpace(defaultMessage)
	}
	if message == "" {
		return CreateAlertResult{Status: "unparseable"}, nil
	}

	recurrence := marshalRecurrence(dec.Recurrence)
	// A one-shot in the past is an error; a recurring rule is fine (the scheduler
	// will advance it to the next future occurrence on the first tick).
	if recurrence == "" && !fireAt.After(time.Now()) {
		return CreateAlertResult{Status: "past"}, nil
	}

	id, err := s.db.CreateAlert(db.Alert{Message: message, NoteID: noteID, FireAt: fireAt, Recurrence: recurrence})
	if err != nil {
		return CreateAlertResult{Status: "error"}, nil
	}
	return CreateAlertResult{
		Status:      "created",
		AlertID:     id,
		Message:     message,
		FireAtLocal: fireAt.In(s.loc).Format(time.RFC3339),
		Recurrence:  recurrence,
	}, nil
}

// parseWhen runs one AI call to turn natural-language timing into an absolute
// fire time (ISO w/ offset) and optional recurrence. The prompt always injects
// the current local time, zone, and language because the model's own clock is
// unreliable. Shared by Create and CreateForNote.
func (s *AlertsService) parseWhen(text, contextMessage string, now time.Time) (alertDecision, error) {
	cfg := s.cfg.Current()
	client := s.newClient(cfg.ResolveAPIKey())
	raw, _, err := client.Complete(context.Background(), cfg.Model, buildAlertPrompt(text, contextMessage, now.In(s.loc), cfg.Language))
	if err != nil {
		return alertDecision{}, err
	}
	var dec alertDecision
	if err := json.Unmarshal([]byte(stripFences(raw)), &dec); err != nil {
		return alertDecision{}, err
	}
	return dec, nil
}

func buildAlertPrompt(text, contextMessage string, now time.Time, language string) []ai.Message {
	zoneName, offsetSec := now.Zone()
	offsetH := offsetSec / 3600
	noteLine := ""
	if contextMessage != "" {
		noteLine = fmt.Sprintf("\nO alerta refere-se à nota: %q. Use-a como \"message\" se o usuário não disser outro texto.", contextMessage)
	}
	system := fmt.Sprintf(`Você converte um lembrete em linguagem natural em JSON estruturado.
Data e hora locais atuais: %s. Fuso: %s (UTC%+d). Idioma do usuário: %s.
Resolva datas/horas relativas ("amanhã", "sexta", "às 9h") em relação a ESTE momento.%s
Responda APENAS com JSON, sem cercas:
{"message":"<texto curto do lembrete>","fire_at":"<ISO 8601 com offset, ex 2026-06-26T09:00:00%+03d:00>","recurrence":<null ou {"freq":"daily|weekly|monthly|yearly","interval":1,"weekday":"mon","time":"09:00"}>}
"recurrence" é null para lembrete único; "weekday" só em "weekly". Nunca invente; se faltar horário, assuma 09:00 local.`,
		now.Format("2006-01-02 15:04 (Monday)"), zoneName, offsetH, language, noteLine, offsetH)
	return []ai.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: text},
	}
}
