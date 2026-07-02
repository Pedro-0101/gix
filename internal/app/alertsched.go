package app

import (
	"strings"
	"time"

	"gix/internal/app/log"
	"gix/internal/app/winnotify"
)

// ScheduledAlert é a forma mínima que o frontend envia ao serviço (vinda de
// AlertsService.list()/create*). FireAt é RFC3339.
type ScheduledAlert struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
	FireAt  string `json:"fireAt"`
	Status  string `json:"status"`
}

// AlertSchedulerService arma/cancela toasts agendados do Windows espelhando os
// alertas do servidor. O diff (desejado vs armado) é testável; o I/O nativo fica
// atrás de winnotify.Notifier. Os erros do Notifier são repassados ao chamador;
// cabe a ele decidir se aborta ou continua — agendar é best-effort e não deve
// bloquear o fluxo de alertas.
type AlertSchedulerService struct {
	n winnotify.Notifier
}

func NewAlertSchedulerService(n winnotify.Notifier) *AlertSchedulerService {
	return &AlertSchedulerService{n: n}
}

// desired converte um ScheduledAlert numa Occurrence futura, ou (zero,false) se
// não-ativo/pending ou no passado.
func desired(a ScheduledAlert) (winnotify.Occurrence, bool) {
	status := strings.ToLower(strings.TrimSpace(a.Status))
	if status != "active" && status != "pending" {
		log.Printf("alertsched.desired: id=%d REJECTED status=%q (want active or pending)", a.ID, a.Status)
		return winnotify.Occurrence{}, false
	}
	t, err := time.Parse(time.RFC3339, a.FireAt)
	if err != nil {
		log.Printf("alertsched.desired: id=%d REJECTED parse error fireAt=%q: %v", a.ID, a.FireAt, err)
		return winnotify.Occurrence{}, false
	}
	now := time.Now()
	if !t.After(now) {
		log.Printf("alertsched.desired: id=%d REJECTED fireAt=%s in the past (now=%s)", a.ID, t.Format(time.RFC3339), now.Format(time.RFC3339))
		return winnotify.Occurrence{}, false
	}
	log.Printf("alertsched.desired: id=%d ACCEPTED fireAt=%s message=%q", a.ID, t.Format(time.RFC3339), a.Message)
	return winnotify.Occurrence{AlertID: a.ID, FireAt: t, Message: a.Message}, true
}

// ArmOne arma a próxima ocorrência de um alerta (no-op se passado/não-ativo).
func (s *AlertSchedulerService) ArmOne(a ScheduledAlert) error {
	log.Printf("alertsched.ArmOne: called id=%d status=%q fireAt=%q", a.ID, a.Status, a.FireAt)
	occ, ok := desired(a)
	if !ok {
		return nil
	}
	err := s.n.Arm(occ)
	if err != nil {
		log.Printf("alertsched.ArmOne: id=%d ERROR arming: %v", a.ID, err)
	} else {
		log.Printf("alertsched.ArmOne: id=%d ARMED ok", a.ID)
	}
	return err
}

// CancelOne remove qualquer ocorrência agendada do alerta.
func (s *AlertSchedulerService) CancelOne(alertID int64) error {
	log.Printf("alertsched.CancelOne: called id=%d", alertID)
	err := s.n.CancelByAlert(alertID)
	if err != nil {
		log.Printf("alertsched.CancelOne: id=%d ERROR: %v", alertID, err)
	} else {
		log.Printf("alertsched.CancelOne: id=%d CANCELLED ok", alertID)
	}
	return err
}

// Reconcile alinha o conjunto agendado do SO com a lista de alertas: cancela o
// que sumiu ou mudou de horário e arma o que falta. Uma ocorrência por alerta.
func (s *AlertSchedulerService) Reconcile(alerts []ScheduledAlert) error {
	log.Printf("alertsched.Reconcile: called with %d alerts", len(alerts))
	for i, a := range alerts {
		log.Printf("alertsched.Reconcile: input[%d] id=%d status=%q fireAt=%q message=%q", i, a.ID, a.Status, a.FireAt, a.Message)
	}
	armed, err := s.n.ListArmed()
	if err != nil {
		log.Printf("alertsched.Reconcile: ListArmed ERROR: %v", err)
		return err
	}
	log.Printf("alertsched.Reconcile: ListArmed returned %d armed", len(armed))
	armedBy := make(map[int64]int64, len(armed))
	for _, k := range armed {
		armedBy[k.AlertID] = k.FireAtUnix
	}
	want := make(map[int64]winnotify.Occurrence)
	for _, a := range alerts {
		if occ, ok := desired(a); ok {
			want[a.ID] = occ
		}
	}
	log.Printf("alertsched.Reconcile: want %d alerts after desired() filter", len(want))
	for id, unix := range armedBy {
		w, ok := want[id]
		if !ok || w.FireAt.Unix() != unix {
			log.Printf("alertsched.Reconcile: cancelling stale id=%d (in_want=%v unix_match=%v)", id, ok, ok && w.FireAt.Unix() == unix)
			if err := s.n.CancelByAlert(id); err != nil {
				log.Printf("alertsched.Reconcile: cancel id=%d ERROR: %v", id, err)
				return err
			}
		}
	}
	for id, occ := range want {
		if unix, ok := armedBy[id]; !ok || unix != occ.FireAt.Unix() {
			log.Printf("alertsched.Reconcile: arming id=%d fireAt=%s (already_armed=%v)", id, occ.FireAt.Format(time.RFC3339), ok)
			if err := s.n.Arm(occ); err != nil {
				log.Printf("alertsched.Reconcile: arm id=%d ERROR: %v", id, err)
				return err
			}
			log.Printf("alertsched.Reconcile: arm id=%d OK", id)
		}
	}
	log.Printf("alertsched.Reconcile: done")
	return nil
}
