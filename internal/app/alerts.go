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

// --- management (also called by the toast action buttons) ---

// List returns alerts the manager shows: pending and done, fire_at ascending.
func (s *AlertsService) List() ([]db.Alert, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.ListAlerts("pending", "done")
}

// Cancel soft-cancels an alert (status=cancelled), stopping any recurrence.
func (s *AlertsService) Cancel(id int64) error {
	if s.db == nil {
		return fmt.Errorf("no_db")
	}
	return s.db.SetAlertStatus(id, "cancelled")
}

// Snooze pushes an alert's fire_at forward by `minutes` and keeps it pending.
func (s *AlertsService) Snooze(id int64, minutes int) error {
	if s.db == nil {
		return fmt.Errorf("no_db")
	}
	return s.db.UpdateAlertFireAt(id, time.Now().Add(time.Duration(minutes)*time.Minute))
}

// Done marks an alert done. For a one-shot it's the natural close (idempotent if
// the scheduler already fired it). For a recurring alert it only dismisses the
// current occurrence — the scheduler has already rescheduled the next fire_at,
// so the series continues; cancel the recurrence via Cancel/DeleteAlert instead.
func (s *AlertsService) Done(id int64) error {
	if s.db == nil {
		return fmt.Errorf("no_db")
	}
	a, err := s.db.GetAlert(id)
	if err != nil {
		return err
	}
	if a.Recurrence != "" {
		return nil // recurring: no-op for scheduling
	}
	return s.db.SetAlertStatus(id, "done")
}

// --- scheduler ---

// Run drives the polling loop: an immediate tick on boot (catches alerts that
// came due while the app was closed), then every pollInterval until ctx is done.
func (s *AlertsService) Run(ctx context.Context) {
	s.tick(time.Now())
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(time.Now())
		}
	}
}

// tick fires every due alert once, then reschedules recurring ones to their next
// future occurrence and marks one-shots done.
func (s *AlertsService) tick(now time.Time) {
	if s.db == nil {
		return
	}
	due, err := s.db.DueAlerts(now)
	if err != nil {
		return
	}
	for _, a := range due {
		s.dispatch(a)
		if rule, ok := parseRecurrence(a.Recurrence); ok {
			next := proximoFireAt(rule, a.FireAt.In(s.loc), now.In(s.loc))
			_ = s.db.UpdateAlertFireAt(a.ID, next.UTC())
			continue
		}
		_ = s.db.SetAlertStatus(a.ID, "done")
	}
}

// dispatch notifies the user of one fired alert via two parallel paths so the
// alert is never silently lost: always emit alert:fired (the overlay shows a
// card if open), and send a native toast with action buttons. If the toast
// can't be sent (e.g. `wails dev`, unsigned app, no permission), fall back to
// showing the overlay window.
func (s *AlertsService) dispatch(a db.Alert) {
	if s.emit != nil {
		s.emit("alert:fired", alertFiredPayload{ID: a.ID, Message: a.Message, NoteID: a.NoteID})
	}
	opts := notifications.NotificationOptions{
		ID:         fmt.Sprintf("%d", a.ID),
		Title:      a.Message,
		Body:       a.FireAt.In(s.loc).Format("15:04"),
		CategoryID: alertCategoryID,
		Data:       map[string]interface{}{"id": a.ID},
	}
	if s.notifier == nil || s.notifier.SendNotificationWithActions(opts) != nil {
		if s.onShow != nil {
			s.onShow()
		}
	}
}

// RegisterCategory registers the toast action buttons. Called once at boot.
func (s *AlertsService) RegisterCategory() {
	if s.notifier == nil {
		return
	}
	_ = s.notifier.RegisterNotificationCategory(notifications.NotificationCategory{
		ID: alertCategoryID,
		Actions: []notifications.NotificationAction{
			{ID: "snooze", Title: "Adiar 10 min"},
			{ID: "done", Title: "Concluir"},
		},
	})
}

// alertFiredPayload is the alert:fired event body the frontend consumes.
type alertFiredPayload struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
	NoteID  *int64 `json:"noteId"`
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
