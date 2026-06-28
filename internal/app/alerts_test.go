package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"gix/internal/db"
)

type fakeNotifier struct {
	sent       []notifications.NotificationOptions
	sendErr    error
	categories []notifications.NotificationCategory
}

func (f *fakeNotifier) SendNotificationWithActions(o notifications.NotificationOptions) error {
	f.sent = append(f.sent, o)
	return f.sendErr
}
func (f *fakeNotifier) RegisterNotificationCategory(c notifications.NotificationCategory) error {
	f.categories = append(f.categories, c)
	return nil
}

func newAlertsSvc(t *testing.T, d *db.Database, fake Completer) *AlertsService {
	t.Helper()
	t.Setenv("AppData", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "k")
	svc := NewAlertsService(NewConfigService(), d, func(string) Completer { return fake }, func(string, any) {}, func() {}, nil)
	svc.loc = time.UTC
	return svc
}

func TestCreateOneShotAlert(t *testing.T) {
	d := alertsTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"message":"ligar pro médico","fire_at":"2099-01-02T09:00:00-03:00","recurrence":null}`,
	}}
	svc := newAlertsSvc(t, d, fake)

	res, err := svc.Create("ligar pro médico amanhã às 9h")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if res.Status != "created" || res.AlertID == 0 || res.Message != "ligar pro médico" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Recurrence != "" {
		t.Fatalf("one-shot should have empty recurrence, got %q", res.Recurrence)
	}
	stored, _ := d.GetAlert(res.AlertID)
	if !stored.FireAt.Equal(time.Date(2099, 1, 2, 12, 0, 0, 0, time.UTC)) { // -03:00 -> UTC
		t.Fatalf("fire_at not stored in UTC: %v", stored.FireAt)
	}
}

func TestCreateRecurringAlert(t *testing.T) {
	d := alertsTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"message":"academia","fire_at":"2099-06-01T08:00:00-03:00","recurrence":{"freq":"weekly","interval":1,"weekday":"mon","time":"08:00"}}`,
	}}
	svc := newAlertsSvc(t, d, fake)

	res, err := svc.Create("toda segunda 8h academia")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if res.Status != "created" {
		t.Fatalf("expected created, got %+v", res)
	}
	if _, ok := parseRecurrence(res.Recurrence); !ok {
		t.Fatalf("expected a recurrence rule, got %q", res.Recurrence)
	}
}

func TestCreatePastOneShotRejected(t *testing.T) {
	d := alertsTestDB(t)
	fake := &fakeCompleter{responses: []string{
		`{"message":"x","fire_at":"2000-01-01T09:00:00-03:00","recurrence":null}`,
	}}
	svc := newAlertsSvc(t, d, fake)

	res, _ := svc.Create("ontem")
	if res.Status != "past" {
		t.Fatalf("expected past, got %+v", res)
	}
	if all, _ := d.ListAlerts(); len(all) != 0 {
		t.Fatalf("past alert must not be stored: %+v", all)
	}
}

func TestCreateUnparseable(t *testing.T) {
	d := alertsTestDB(t)
	fake := &fakeCompleter{responses: []string{"desculpe, não entendi"}}
	svc := newAlertsSvc(t, d, fake)

	res, _ := svc.Create("???")
	if res.Status != "unparseable" {
		t.Fatalf("expected unparseable, got %+v", res)
	}
}

func TestCreateNoAPIKey(t *testing.T) {
	d := alertsTestDB(t)
	t.Setenv("AppData", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "")
	svc := NewAlertsService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} }, func(string, any) {}, func() {}, nil)
	svc.loc = time.UTC

	res, _ := svc.Create("amanhã")
	if res.Status != "no_api_key" {
		t.Fatalf("expected no_api_key, got %+v", res)
	}
}

func TestCreateForNoteDefaultsMessageToNoteTitle(t *testing.T) {
	d := alertsTestDB(t)
	noteID, _ := d.CreateNote("Renovar passaporte", "ir ao cartório", nil, nil, 0)
	// AI returns only timing; empty message -> service falls back to note title.
	fake := &fakeCompleter{responses: []string{
		`{"message":"","fire_at":"2099-03-10T10:00:00-03:00","recurrence":null}`,
	}}
	svc := newAlertsSvc(t, d, fake)

	res, err := svc.CreateForNote(noteID, "dia 10 às 10h")
	if err != nil {
		t.Fatalf("CreateForNote: %v", err)
	}
	if res.Status != "created" || res.Message != "Renovar passaporte" {
		t.Fatalf("expected message defaulted to note title, got %+v", res)
	}
	stored, _ := d.GetAlert(res.AlertID)
	if stored.NoteID == nil || *stored.NoteID != noteID {
		t.Fatalf("expected note_id link, got %+v", stored)
	}
}

func TestCreateProposedStoresFutureAndRejectsPast(t *testing.T) {
	d := alertsTestDB(t)
	svc := newAlertsSvc(t, d, &fakeCompleter{})

	res, err := svc.CreateProposed("dentista", "2099-05-05T09:00:00-03:00", "", nil)
	if err != nil {
		t.Fatalf("CreateProposed: %v", err)
	}
	if res.Status != "created" || res.Message != "dentista" || res.AlertID == 0 {
		t.Fatalf("esperava created, veio %+v", res)
	}

	past, _ := svc.CreateProposed("velho", "2000-01-01T09:00:00-03:00", "", nil)
	if past.Status != "past" {
		t.Fatalf("esperava past, veio %+v", past)
	}
	if all, _ := d.ListAlerts(); len(all) != 1 {
		t.Fatalf("só o futuro deveria estar gravado, veio %d", len(all))
	}
}

func TestCreateProposedLinksNoteAndKeepsRecurrence(t *testing.T) {
	d := alertsTestDB(t)
	noteID, _ := d.CreateNote("Pagar conta", "boleto", nil, nil, 0)
	svc := newAlertsSvc(t, d, &fakeCompleter{})

	res, _ := svc.CreateProposed("Pagar conta", "2000-01-01T08:00:00-03:00",
		`{"freq":"monthly","interval":1}`, &noteID)
	if res.Status != "created" {
		t.Fatalf("recorrente no passado deveria gravar, veio %+v", res)
	}
	stored, _ := d.GetAlert(res.AlertID)
	if stored.NoteID == nil || *stored.NoteID != noteID {
		t.Fatalf("esperava vínculo com a nota, veio %+v", stored)
	}
}

func TestFutureOrRecurring(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	if futureOrRecurring("2020-01-01T09:00:00-03:00", "", now) {
		t.Fatal("passado sem recorrência não deveria valer")
	}
	if !futureOrRecurring("2020-01-01T09:00:00-03:00", `{"freq":"daily","interval":1}`, now) {
		t.Fatal("recorrente deveria valer mesmo no passado")
	}
	if !futureOrRecurring("2099-01-01T09:00:00-03:00", "", now) {
		t.Fatal("futuro deveria valer")
	}
	if futureOrRecurring("data ruim", "", now) {
		t.Fatal("data inválida não deveria valer")
	}
}

func TestRegisterCategoryInstallsActionButtons(t *testing.T) {
	d := alertsTestDB(t)
	fn := &fakeNotifier{}
	svc := NewAlertsService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} }, func(string, any) {}, nil, fn)

	svc.RegisterCategory()

	if len(fn.categories) != 1 {
		t.Fatalf("expected one category registered, got %d", len(fn.categories))
	}
	got := fn.categories[0].Actions
	if len(got) != len(alertActions) {
		t.Fatalf("category should expose every registry action, got %d want %d", len(got), len(alertActions))
	}
	for i, a := range alertActions {
		if got[i].ID != a.id || got[i].Title != a.title {
			t.Fatalf("button %d = %+v, want id=%q title=%q", i, got[i], a.id, a.title)
		}
	}
}

func TestHandleNotificationResponseRoutesActions(t *testing.T) {
	d := alertsTestDB(t)
	now := time.Now().UTC()
	id, _ := d.CreateAlert(db.Alert{Message: "x", FireAt: now.Add(time.Hour)})

	shown := 0
	var opened []any
	svc := NewAlertsService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} },
		func(name string, data any) {
			if name == "alert:open" {
				opened = append(opened, data)
			}
		}, func() { shown++ }, nil)
	svc.loc = time.UTC

	resp := func(action string) notifications.NotificationResult {
		return notifications.NotificationResult{
			Response: notifications.NotificationResponse{ID: fmt.Sprintf("%d", id), ActionIdentifier: action},
		}
	}

	// snooze: pushes fire_at forward, alert stays pending, no overlay.
	svc.HandleNotificationResponse(resp("snooze"))
	if a, _ := d.GetAlert(id); a.Status != "pending" || !a.FireAt.After(now.Add(9*time.Minute)) {
		t.Fatalf("snooze action should reschedule and keep pending, got %+v", a)
	}

	// done: marks the one-shot done.
	svc.HandleNotificationResponse(resp("done"))
	if a, _ := d.GetAlert(id); a.Status != "done" {
		t.Fatalf("done action should mark done, got %q", a.Status)
	}

	// default (toast body, empty action id): opens the overlay on the note.
	svc.HandleNotificationResponse(resp(""))
	if shown != 1 || len(opened) != 1 {
		t.Fatalf("default action should open overlay once, shown=%d opened=%d", shown, len(opened))
	}

	// an error result is ignored.
	svc.HandleNotificationResponse(notifications.NotificationResult{Error: fmt.Errorf("boom")})
	if shown != 1 {
		t.Fatalf("error result must be a no-op, shown=%d", shown)
	}
}

func alertsTestDB(t *testing.T) *db.Database {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/notes.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestTickFiresOneShotAndMarksDone(t *testing.T) {
	d := alertsTestDB(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	id, _ := d.CreateAlert(db.Alert{Message: "remédio", FireAt: now.Add(-time.Minute)})

	fn := &fakeNotifier{}
	var fired []any
	svc := NewAlertsService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} },
		func(name string, data any) {
			if name == "alert:fired" {
				fired = append(fired, data)
			}
		}, nil, fn)
	svc.loc = time.UTC

	svc.tick(now)

	if len(fn.sent) != 1 {
		t.Fatalf("expected one toast, got %d", len(fn.sent))
	}
	if len(fired) != 1 {
		t.Fatalf("expected one alert:fired event, got %d", len(fired))
	}
	got, _ := d.GetAlert(id)
	if got.Status != "done" {
		t.Fatalf("one-shot should be done after firing, got %q", got.Status)
	}
}

func TestTickRecurringFiresOnceAndReschedulesFuture(t *testing.T) {
	d := alertsTestDB(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	// Daily alert last due 3 days ago: must fire exactly once and reschedule ahead.
	id, _ := d.CreateAlert(db.Alert{
		Message:    "academia",
		FireAt:     now.AddDate(0, 0, -3).Truncate(time.Hour),
		Recurrence: `{"freq":"daily","interval":1}`,
	})

	fn := &fakeNotifier{}
	svc := NewAlertsService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} }, func(string, any) {}, nil, fn)
	svc.loc = time.UTC

	svc.tick(now)

	if len(fn.sent) != 1 {
		t.Fatalf("recurring catch-up must fire exactly once, got %d", len(fn.sent))
	}
	got, _ := d.GetAlert(id)
	if got.Status != "pending" {
		t.Fatalf("recurring alert should stay pending, got %q", got.Status)
	}
	if !got.FireAt.After(now) {
		t.Fatalf("recurring alert should be rescheduled to the future, got %v", got.FireAt)
	}
}

func TestDispatchFallsBackToOverlayOnToastError(t *testing.T) {
	d := alertsTestDB(t)
	shown := 0
	fn := &fakeNotifier{sendErr: fmt.Errorf("toast unavailable")}
	svc := NewAlertsService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} },
		func(string, any) {}, func() { shown++ }, fn)
	svc.loc = time.UTC

	svc.dispatch(db.Alert{ID: 1, Message: "x"})
	if shown != 1 {
		t.Fatalf("expected overlay fallback when toast fails, shown=%d", shown)
	}
}

func TestSnoozeDoneCancel(t *testing.T) {
	d := alertsTestDB(t)
	now := time.Now().UTC()
	id, _ := d.CreateAlert(db.Alert{Message: "x", FireAt: now.Add(time.Hour)})
	svc := NewAlertsService(NewConfigService(), d, func(string) Completer { return &fakeCompleter{} }, func(string, any) {}, nil, nil)
	svc.loc = time.UTC

	if err := svc.Snooze(id, 10); err != nil {
		t.Fatalf("Snooze: %v", err)
	}
	a, _ := d.GetAlert(id)
	if a.Status != "pending" || !a.FireAt.After(now.Add(9*time.Minute)) {
		t.Fatalf("snooze should push fire_at ~10min and keep pending: %+v", a)
	}
	if err := svc.Done(id); err != nil {
		t.Fatalf("Done: %v", err)
	}
	if a, _ := d.GetAlert(id); a.Status != "done" {
		t.Fatalf("Done should set status done, got %q", a.Status)
	}
	id2, _ := d.CreateAlert(db.Alert{Message: "y", FireAt: now.Add(time.Hour)})
	if err := svc.Cancel(id2); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if a, _ := d.GetAlert(id2); a.Status != "cancelled" {
		t.Fatalf("Cancel should set status cancelled, got %q", a.Status)
	}
}
