package hotkey

import (
	"runtime"
	"time"
)

// Start inicia o listener de hotkey global para o SO atual.
func Start(openKey string, intervalMs int, onTrigger func()) {
	switch runtime.GOOS {
	case "windows":
		go startWindowsHook(openKey, intervalMs, onTrigger)
	case "linux":
		go startLinuxHook(openKey, intervalMs, onTrigger)
	}
}

// DoublePressDetector detecta duplo-pressionamento de tecla dentro de um intervalo.
type DoublePressDetector struct {
	lastPress time.Time
	fn        func()
	interval  time.Duration
}

// Press registra um pressionamento e dispara fn se for um duplo-pressionamento.
func (d *DoublePressDetector) Press() {
	now := time.Now()
	if !d.lastPress.IsZero() && now.Sub(d.lastPress) <= d.interval {
		d.lastPress = time.Time{}
		if d.fn != nil {
			d.fn()
		}
		return
	}
	d.lastPress = now
}
