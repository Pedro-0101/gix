package app

import (
	"testing"
	"time"

	"gix/internal/db"
)

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

func alertsTestDB(t *testing.T) *db.Database {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/notes.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
