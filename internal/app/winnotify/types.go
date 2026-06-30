// Package winnotify arma toasts agendados do Windows para alertas, de modo que
// o lembrete dispare mesmo com o app fechado e offline. No Windows usa winrt-go;
// nas demais plataformas, um stub no-op (o push do servidor segue cobrindo).
package winnotify

import "time"

// Occurrence é um disparo concreto a ser agendado no SO.
type Occurrence struct {
	AlertID int64
	FireAt  time.Time
	Message string
}

// Key identifica uma ocorrência agendada (tag=alertID, group=fireAtUnix).
type Key struct {
	AlertID    int64
	FireAtUnix int64
}

// Notifier abstrai o agendamento nativo. Real no Windows, no-op fora dele.
type Notifier interface {
	Arm(occ Occurrence) error
	CancelByAlert(alertID int64) error
	ListArmed() ([]Key, error)
}
