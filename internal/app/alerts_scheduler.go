package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"gix/internal/db"
)

// pollInterval is how often the scheduler scans for due alerts.
const pollInterval = 30 * time.Second

// alertCategoryID groups the toast action buttons (snooze / done).
const alertCategoryID = "gix.alert"

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
			if err := s.db.UpdateAlertFireAt(a.ID, next.UTC()); err != nil {
				log.Printf("alerts: reschedule %d: %v", a.ID, err)
			}
			continue
		}
		if err := s.db.SetAlertStatus(a.ID, "done"); err != nil {
			log.Printf("alerts: mark done %d: %v", a.ID, err)
		}
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

// alertFiredPayload is the alert:fired event body the frontend consumes.
type alertFiredPayload struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
	NoteID  *int64 `json:"noteId"`
}
