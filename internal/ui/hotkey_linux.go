//go:build linux

package ui

import (
	"encoding/binary"
	"os"
	"path/filepath"
)

const (
	evKey     = 1
	keySpace  = 57
)

type inputEvent struct {
	sec   int64
	usec  int64
	_type uint16
	code  uint16
	value int32
}

func startLinuxHook(fn func()) {
	detector := &doubleSpaceDetector{fn: fn}

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
		if ev._type == evKey && ev.code == keySpace && ev.value == 1 {
			detector.press()
		}
	}
}

func startWindowsHook(fn func()) {}
