package ui

import (
	"gix/internal/config"
	"runtime"
	"time"
)

func startHotkeyListener(fn func(), cfg *config.Config) {
	switch runtime.GOOS {
	case "windows":
		go startWindowsHook(fn, cfg)
	case "linux":
		go startLinuxHook(fn, cfg)
	}
}

type doublePressDetector struct {
	lastPress time.Time
	fn        func()
	interval  time.Duration
}

func (d *doublePressDetector) press() {
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
