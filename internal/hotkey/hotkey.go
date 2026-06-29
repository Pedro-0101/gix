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
//
// A janela de tempo é rolante: cada toque vale se acontecer dentro de `interval`
// do toque anterior, e a contagem sobe até `target`. Ancorar a janela no
// primeiro toque tornaria o triplo-toque difícil de acertar (os 3 teriam de
// caber num único `interval`); a janela rolante segue o mesmo feel do antigo
// duplo-toque, só que para N toques.
type MultiPressDetector struct {
	lastPress time.Time
	count     int
	fn        func()
	interval  time.Duration
	target    int
	// now permite injetar o relógio nos testes; nil usa time.Now.
	now func() time.Time
}

func (d *MultiPressDetector) clock() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

// Press registra um pressionamento e dispara fn ao atingir o número alvo de
// toques consecutivos, cada um dentro de `interval` do anterior.
func (d *MultiPressDetector) Press() {
	now := d.clock()
	if d.count == 0 || now.Sub(d.lastPress) > d.interval {
		d.count = 1
		d.lastPress = now
		return
	}
	d.count++
	d.lastPress = now
	if d.count >= d.target {
		d.lastPress = time.Time{}
		d.count = 0
		if d.fn != nil {
			d.fn()
		}
	}
}

// PressOther reseta o detector quando uma tecla diferente é pressionada no meio da sequência.
func (d *MultiPressDetector) PressOther() {
	d.lastPress = time.Time{}
	d.count = 0
}
