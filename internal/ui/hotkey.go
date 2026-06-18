package ui

import (
	"runtime"
	"time"
)

func startHotkeyListener(fn func()) {
	switch runtime.GOOS {
	case "windows":
		go startWindowsHook(fn)
	case "linux":
		go startLinuxHook(fn)
	}
}

type doubleSpaceDetector struct {
	lastPress time.Time
	fn        func()
}

func (d *doubleSpaceDetector) press() {
	now := time.Now()
	if !d.lastPress.IsZero() && now.Sub(d.lastPress) <= 500*time.Millisecond {
		d.lastPress = time.Time{}
		if d.fn != nil {
			d.fn()
		}
		return
	}
	d.lastPress = now
}
