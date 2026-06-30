package app

import (
	"testing"
	"time"

	"gix/internal/app/winnotify"
)

// fakeNotifier guarda o estado agendado em memória (uma ocorrência por alerta).
type fakeNotifier struct {
	armed map[int64]int64 // alertID -> fireAtUnix
}

func newFake() *fakeNotifier { return &fakeNotifier{armed: map[int64]int64{}} }

func (f *fakeNotifier) Arm(o winnotify.Occurrence) error {
	f.armed[o.AlertID] = o.FireAt.Unix()
	return nil
}
func (f *fakeNotifier) CancelByAlert(id int64) error { delete(f.armed, id); return nil }
func (f *fakeNotifier) ListArmed() ([]winnotify.Key, error) {
	out := make([]winnotify.Key, 0, len(f.armed))
	for id, u := range f.armed {
		out = append(out, winnotify.Key{AlertID: id, FireAtUnix: u})
	}
	return out, nil
}

func future(d time.Duration) string { return time.Now().Add(d).UTC().Format(time.RFC3339) }
func past(d time.Duration) string   { return time.Now().Add(-d).UTC().Format(time.RFC3339) }

func TestArmOneFuture(t *testing.T) {
	f := newFake()
	s := NewAlertSchedulerService(f)
	if err := s.ArmOne(ScheduledAlert{ID: 1, Message: "x", FireAt: future(time.Hour), Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.armed[1]; !ok {
		t.Fatal("esperava alerta 1 armado")
	}
}

func TestArmOnePastIsNoop(t *testing.T) {
	f := newFake()
	s := NewAlertSchedulerService(f)
	if err := s.ArmOne(ScheduledAlert{ID: 1, FireAt: past(time.Hour), Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if len(f.armed) != 0 {
		t.Fatalf("passado não deve armar: %v", f.armed)
	}
}

func TestCancelOne(t *testing.T) {
	f := newFake()
	f.armed[7] = 123
	s := NewAlertSchedulerService(f)
	if err := s.CancelOne(7); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.armed[7]; ok {
		t.Fatal("esperava alerta 7 cancelado")
	}
}

func TestReconcileArmsCancelsAndUpdates(t *testing.T) {
	f := newFake()
	f.armed[2] = 111            // será removido (não está mais na lista)
	f.armed[3] = 222            // mudou de horário -> cancela e rearma
	s := NewAlertSchedulerService(f)

	soon := future(time.Hour)
	alerts := []ScheduledAlert{
		{ID: 1, FireAt: soon, Status: "active"},          // novo -> arma
		{ID: 3, FireAt: future(2 * time.Hour), Status: "active"}, // mudou
		{ID: 4, FireAt: past(time.Minute), Status: "active"},     // passado -> ignora
		{ID: 5, FireAt: soon, Status: "done"},                    // não-ativo -> ignora
	}
	if err := s.Reconcile(alerts); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.armed[2]; ok {
		t.Fatal("alerta 2 deveria ter sido cancelado")
	}
	if _, ok := f.armed[1]; !ok {
		t.Fatal("alerta 1 deveria ter sido armado")
	}
	want3 := mustUnix(t, future(2*time.Hour))
	if f.armed[3] == 222 || abs(f.armed[3]-want3) > 2 {
		t.Fatalf("alerta 3 deveria ter rearmado para o novo horário, got %d", f.armed[3])
	}
	if _, ok := f.armed[4]; ok {
		t.Fatal("alerta 4 (passado) não deveria estar armado")
	}
	if _, ok := f.armed[5]; ok {
		t.Fatal("alerta 5 (done) não deveria estar armado")
	}
}

func mustUnix(t *testing.T, iso string) int64 {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t.Fatal(err)
	}
	return tm.Unix()
}
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
