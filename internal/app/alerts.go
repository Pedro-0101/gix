package app

import (
	"fmt"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"gix/internal/db"
)

// notifier is the slice of the Wails NotificationService the scheduler needs.
// Injected so tests can fake it (and so a nil notifier degrades to overlay-only).
type notifier interface {
	SendNotificationWithActions(options notifications.NotificationOptions) error
	RegisterNotificationCategory(category notifications.NotificationCategory) error
}

// AlertsService creates reminders (AI-parsed) and fires them on a polling loop.
// Creation/parsing lives in alerts_create.go; the polling loop and toast
// dispatch in alerts_scheduler.go; recurrence rules in recurrence.go.
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

// GetAlertNoteID returns the note_id of an alert, or nil if unlinked.
func (s *AlertsService) GetAlertNoteID(id int64) (*int64, error) {
	if s.db == nil {
		return nil, fmt.Errorf("no_db")
	}
	a, err := s.db.GetAlert(id)
	if err != nil {
		return nil, err
	}
	return a.NoteID, nil
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
