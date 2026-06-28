package app

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// alertAction binds a toast button to the code that runs when the user taps it,
// in one place. The registry below is the single source of truth for both the
// buttons RegisterCategory installs and the dispatch HandleNotificationResponse
// performs, so a new action is one entry here — no second file to edit and no
// switch to extend (open/closed).
type alertAction struct {
	id     string
	title  string
	handle func(s *AlertsService, alertID int64)
}

// alertActions are the toast buttons, in display order. A toast-body click (no
// button) isn't here: it's the default, handled by openOnAlert below.
var alertActions = []alertAction{
	{id: "snooze", title: "Adiar 10 min", handle: func(s *AlertsService, id int64) { _ = s.Snooze(id, 10) }},
	{id: "done", title: "Concluir", handle: func(s *AlertsService, id int64) { _ = s.Done(id) }},
}

// RegisterCategory installs the toast action buttons from the registry. Called
// once at boot.
func (s *AlertsService) RegisterCategory() {
	if s.notifier == nil {
		return
	}
	actions := make([]notifications.NotificationAction, len(alertActions))
	for i, a := range alertActions {
		actions[i] = notifications.NotificationAction{ID: a.id, Title: a.title}
	}
	_ = s.notifier.RegisterNotificationCategory(notifications.NotificationCategory{
		ID:      alertCategoryID,
		Actions: actions,
	})
}

// HandleNotificationResponse routes a toast interaction to its handler: a button
// tap runs the matching registry entry; a toast-body click (the default action,
// which has no button) opens the overlay on the alert's note. Wired to the
// notifier in shell.go.
func (s *AlertsService) HandleNotificationResponse(result notifications.NotificationResult) {
	if result.Error != nil {
		return
	}
	var id int64
	fmt.Sscanf(result.Response.ID, "%d", &id)
	for _, a := range alertActions {
		if a.id == result.Response.ActionIdentifier {
			a.handle(s, id)
			return
		}
	}
	s.openOnAlert(id) // DEFAULT_ACTION: user clicked the toast body
}

// openOnAlert surfaces the overlay and asks the frontend to open the alert's note.
func (s *AlertsService) openOnAlert(id int64) {
	if s.onShow != nil {
		s.onShow()
	}
	noteID, _ := s.GetAlertNoteID(id)
	if s.emit != nil {
		s.emit("alert:open", map[string]any{"id": id, "noteId": noteID})
	}
}
