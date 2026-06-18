//go:build !windows && !linux

package ui

func startWindowsHook(fn func()) {}
func startLinuxHook(fn func()) {}
