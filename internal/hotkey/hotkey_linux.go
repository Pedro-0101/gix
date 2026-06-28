//go:build linux

package hotkey

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"time"
)

var evCodeMap = map[string]uint16{
	"Space":  57,
	"Escape": 1,
	"Tab":    15,
	"Enter":  28,
}

var skipEvCodes = map[uint16]bool{
	29:  true, // KEY_LEFTCTRL
	42:  true, // KEY_LEFTSHIFT
	56:  true, // KEY_LEFTALT
	97:  true, // KEY_RIGHTCTRL
	54:  true, // KEY_RIGHTSHIFT
	100: true, // KEY_RIGHTALT
	125: true, // KEY_LEFTMETA
	126: true, // KEY_RIGHTMETA
	58:  true, // KEY_CAPSLOCK
}

const (
	evKey = 1
)

type inputEvent struct {
	sec   int64
	usec  int64
	_type uint16
	code  uint16
	value int32
}

func startWindowsHook(openKey string, intervalMs int, pressCount int, fn func()) {}

func startLinuxHook(openKey string, intervalMs int, pressCount int, fn func()) {
	keyCode := evCodeMap[openKey]
	detector := &MultiPressDetector{
		fn:       fn,
		interval: time.Duration(intervalMs) * time.Millisecond,
		target:   pressCount,
	}

	devices, err := filepath.Glob("/dev/input/event*")
	if err != nil || len(devices) == 0 {
		return
	}

	eventCh := make(chan inputEvent, 64)

	for _, dev := range devices {
		f, err := os.Open(dev)
		if err != nil {
			continue
		}

		go func(file *os.File) {
			defer file.Close()
			var buf [24]byte
			for {
				_, err := file.Read(buf[:])
				if err != nil {
					return
				}
				eventCh <- inputEvent{
					sec:   int64(binary.LittleEndian.Uint64(buf[0:8])),
					usec:  int64(binary.LittleEndian.Uint64(buf[8:16])),
					_type: binary.LittleEndian.Uint16(buf[16:18]),
					code:  binary.LittleEndian.Uint16(buf[18:20]),
					value: int32(binary.LittleEndian.Uint32(buf[20:24])),
				}
			}
		}(f)
	}

	for ev := range eventCh {
		if ev._type != evKey || ev.value != 1 {
			continue
		}
		if ev.code == keyCode {
			detector.Press()
		} else if !skipEvCodes[ev.code] {
			detector.PressOther()
		}
	}
}

// Apply reaplica a configuração de hotkey em runtime (Linux: requer reinicialização do app).
func Apply(openKey string, intervalMs int, pressCount int) {
	// Linux hook restart would need to re-read devices
	// For now, hotkey config changes require app restart on Linux
}
