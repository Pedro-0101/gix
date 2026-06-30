package app

import (
	"time"

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
// não-ativo ou no passado.
func desired(a ScheduledAlert) (winnotify.Occurrence, bool) {
	if a.Status != "active" {
		return winnotify.Occurrence{}, false
	}
	t, err := time.Parse(time.RFC3339, a.FireAt)
	if err != nil || !t.After(time.Now()) {
		return winnotify.Occurrence{}, false
	}
	return winnotify.Occurrence{AlertID: a.ID, FireAt: t, Message: a.Message}, true
}

// ArmOne arma a próxima ocorrência de um alerta (no-op se passado/não-ativo).
func (s *AlertSchedulerService) ArmOne(a ScheduledAlert) error {
	occ, ok := desired(a)
	if !ok {
		return nil
	}
	return s.n.Arm(occ)
}

// CancelOne remove qualquer ocorrência agendada do alerta.
func (s *AlertSchedulerService) CancelOne(alertID int64) error {
	return s.n.CancelByAlert(alertID)
}

// Reconcile alinha o conjunto agendado do SO com a lista de alertas: cancela o
// que sumiu ou mudou de horário e arma o que falta. Uma ocorrência por alerta.
func (s *AlertSchedulerService) Reconcile(alerts []ScheduledAlert) error {
	armed, err := s.n.ListArmed()
	if err != nil {
		return err
	}
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
	for id, unix := range armedBy {
		w, ok := want[id]
		if !ok || w.FireAt.Unix() != unix {
			if err := s.n.CancelByAlert(id); err != nil {
				return err
			}
		}
	}
	for id, occ := range want {
		if unix, ok := armedBy[id]; !ok || unix != occ.FireAt.Unix() {
			if err := s.n.Arm(occ); err != nil {
				return err
			}
		}
	}
	return nil
}
