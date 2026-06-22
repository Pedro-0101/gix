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

func startWindowsHook(openKey string, intervalMs int, fn func()) {}

func startLinuxHook(openKey string, intervalMs int, fn func()) {
	keyCode := evCodeMap[openKey]
	detector := &DoublePressDetector{
		fn:       fn,
		interval: time.Duration(intervalMs) * time.Millisecond,
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
		if ev._type == evKey && ev.code == keyCode && ev.value == 1 {
			detector.Press()
		}
	}
}

// Apply reaplica a configuração de hotkey em runtime (Linux: requer reinicialização do app).
func Apply(openKey string, intervalMs int) {
	// Linux hook restart would need to re-read devices
	// For now, hotkey config changes require app restart on Linux
}
