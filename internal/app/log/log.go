// Package log provides a minimal file logger for debugging the alert scheduling
// flow. Writes to %LOCALAPPDATA%\gix\gix.log with timestamps.
package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu  sync.Mutex
	f   *os.File
)

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	p := filepath.Join(dir, "gix")
	_ = os.MkdirAll(p, 0755)
	fp := filepath.Join(p, "gix.log")
	f, _ = os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if f != nil {
		Printf("=== gix log started %s ===", time.Now().Format(time.RFC3339))
	}
}

func Printf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if f == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(f, ts+" "+format+"\n", args...)
}
