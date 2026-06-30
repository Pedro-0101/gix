//go:build windows

package winnotify

import (
	"os"
	"testing"
	"time"
)

// TestSmoke: exercício programático do Arm/ListArmed/CancelByAlert.
// Executa SOMENTE se WINNOTIFY_SMOKE=1 estiver setado para não poluir o CI.
// Não espera pelo disparo visual — apenas verifica que a API do Windows
// aceita e cancela o agendamento.
func TestSmoke(t *testing.T) {
	if os.Getenv("WINNOTIFY_SMOKE") != "1" {
		t.Skip("set WINNOTIFY_SMOKE=1 to run")
	}

	n := New()

	const alertID int64 = 9999
	fireAt := time.Now().Add(2 * time.Minute)

	// 1. Arm
	if err := n.Arm(Occurrence{AlertID: alertID, FireAt: fireAt, Message: "teste gix smoke"}); err != nil {
		t.Fatalf("Arm: %v", err)
	}

	// 2. ListArmed deve conter a chave
	keys, err := n.ListArmed()
	if err != nil {
		t.Fatalf("ListArmed: %v", err)
	}
	found := false
	for _, k := range keys {
		if k.AlertID == alertID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ListArmed: esperava alertID=%d na lista, obteve %v", alertID, keys)
	}

	// 3. CancelByAlert deve remover
	if err := n.CancelByAlert(alertID); err != nil {
		t.Fatalf("CancelByAlert: %v", err)
	}

	// 4. ListArmed não deve conter mais a chave
	keys, err = n.ListArmed()
	if err != nil {
		t.Fatalf("ListArmed pós-cancel: %v", err)
	}
	for _, k := range keys {
		if k.AlertID == alertID {
			t.Fatalf("CancelByAlert não removeu alertID=%d; lista=%v", alertID, keys)
		}
	}

	t.Log("smoke OK: Arm → ListArmed → CancelByAlert confirmados via API Windows")
}
