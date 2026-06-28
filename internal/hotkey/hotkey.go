package hotkey

import (
	"runtime"
	"time"
)

// Start inicia o listener de hotkey global para o SO atual.
// pressCount define quantos pressionamentos dentro do interval disparam onTrigger (2 = duplo, 3 = triplo).
func Start(openKey string, intervalMs int, pressCount int, onTrigger func()) {
	switch runtime.GOOS {
	case "windows":
		go startWindowsHook(openKey, intervalMs, pressCount, onTrigger)
	case "linux":
		go startLinuxHook(openKey, intervalMs, pressCount, onTrigger)
	}
}

// MultiPressDetector detecta N pressionamentos de tecla dentro de um intervalo.
type MultiPressDetector struct {
	firstPress time.Time
	count      int
	fn         func()
	interval   time.Duration
	target     int
}

// Press registra um pressionamento e dispara fn se atingir o número alvo dentro do intervalo.
func (d *MultiPressDetector) Press() {
	now := time.Now()
	if d.firstPress.IsZero() || now.Sub(d.firstPress) > d.interval {
		d.firstPress = now
		d.count = 1
		return
	}
	d.count++
	if d.count >= d.target {
		d.firstPress = time.Time{}
		d.count = 0
		if d.fn != nil {
			d.fn()
		}
	}
}

// PressOther reseta o detector quando uma tecla diferente é pressionada no meio da sequência.
func (d *MultiPressDetector) PressOther() {
	d.firstPress = time.Time{}
	d.count = 0
}
