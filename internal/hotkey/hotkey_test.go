package hotkey

import (
	"testing"
	"time"
)

// newDetector monta um detector com relógio controlável para timing determinístico.
func newDetector(target int, interval time.Duration, clock *time.Time, fired *int) *MultiPressDetector {
	return &MultiPressDetector{
		fn:       func() { *fired++ },
		interval: interval,
		target:   target,
		now:      func() time.Time { return *clock },
	}
}

// Triplo-toque num ritmo natural (300ms entre toques, ~600ms no total) deve
// disparar. A regressão ancorava a janela no primeiro toque, exigindo os 3
// dentro de 500ms — o que falhava aqui.
func TestTriplePressRollingWindowFires(t *testing.T) {
	clock := time.Unix(0, 0)
	fired := 0
	d := newDetector(3, 500*time.Millisecond, &clock, &fired)

	d.Press()                                 // t=0
	clock = clock.Add(300 * time.Millisecond) // t=300
	d.Press()
	clock = clock.Add(300 * time.Millisecond) // t=600
	d.Press()

	if fired != 1 {
		t.Fatalf("triplo-toque a 300ms de intervalo deveria disparar 1x, disparou %d", fired)
	}
}

// Gap maior que o intervalo reinicia a sequência: não dispara.
func TestPressTooSlowDoesNotFire(t *testing.T) {
	clock := time.Unix(0, 0)
	fired := 0
	d := newDetector(3, 500*time.Millisecond, &clock, &fired)

	d.Press()                                 // t=0
	clock = clock.Add(600 * time.Millisecond) // t=600 (gap > 500)
	d.Press()
	clock = clock.Add(100 * time.Millisecond) // t=700
	d.Press()

	if fired != 0 {
		t.Fatalf("toques lentos não deveriam disparar, disparou %d", fired)
	}
}

// Duplo-toque (count=2) mantém o comportamento de antes: dois toques dentro do
// intervalo disparam.
func TestDoublePressFires(t *testing.T) {
	clock := time.Unix(0, 0)
	fired := 0
	d := newDetector(2, 500*time.Millisecond, &clock, &fired)

	d.Press()
	clock = clock.Add(200 * time.Millisecond)
	d.Press()

	if fired != 1 {
		t.Fatalf("duplo-toque deveria disparar 1x, disparou %d", fired)
	}
}

// Uma tecla diferente no meio da sequência reseta a contagem.
func TestPressOtherResets(t *testing.T) {
	clock := time.Unix(0, 0)
	fired := 0
	d := newDetector(3, 500*time.Millisecond, &clock, &fired)

	d.Press()
	clock = clock.Add(100 * time.Millisecond)
	d.Press()
	d.PressOther() // reseta
	clock = clock.Add(100 * time.Millisecond)
	d.Press()
	clock = clock.Add(100 * time.Millisecond)
	d.Press()

	if fired != 0 {
		t.Fatalf("reset no meio deveria impedir disparo, disparou %d", fired)
	}
}
