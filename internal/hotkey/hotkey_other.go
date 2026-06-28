//go:build !windows && !linux

package hotkey

func startWindowsHook(openKey string, intervalMs int, pressCount int, fn func()) {}
func startLinuxHook(openKey string, intervalMs int, pressCount int, fn func()) {}

// Apply is a no-op on unsupported platforms.
func Apply(openKey string, intervalMs int, pressCount int) {}
