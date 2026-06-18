//go:build !windows && !linux

package ui

import "gix/internal/config"

func startWindowsHook(fn func(), cfg *config.Config) {}
func startLinuxHook(fn func(), cfg *config.Config) {}
func applyHotkeyConfig(cfg *config.Config) {}
