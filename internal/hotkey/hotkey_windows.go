//go:build windows

package hotkey

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	hookUser32              = syscall.NewLazyDLL("user32.dll")
	procSetWindowsHookEx    = hookUser32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = hookUser32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = hookUser32.NewProc("UnhookWindowsHookEx")
	procGetMessageW         = hookUser32.NewProc("GetMessageW")
	procTranslateMessage    = hookUser32.NewProc("TranslateMessage")
	procDispatchMessageW    = hookUser32.NewProc("DispatchMessageW")

	hookKernel32      = syscall.NewLazyDLL("kernel32.dll")
	procRtlMoveMemory = hookKernel32.NewProc("RtlMoveMemory")
)

const (
	whKeyboardLL = 13
	wmKeydown    = 0x0100
)

var vkCodeMap = map[string]uint32{
	"Space":  0x20,
	"Escape": 0x1B,
	"Tab":    0x09,
	"Enter":  0x0D,
}

type kbdLLHookStruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

var (
	hookCallback    uintptr
	hookHandle      uintptr
	winHookDetector *DoublePressDetector
	winHookKeyCode  uint32 = 0x20
)

func readKbdLLVkCode(lParam uintptr) uint32 {
	var kbd kbdLLHookStruct
	procRtlMoveMemory.Call(
		uintptr(unsafe.Pointer(&kbd)),
		lParam,
		unsafe.Sizeof(kbd),
	)
	return kbd.vkCode
}

func winLowLevelKeyboardProc(code int, wParam uintptr, lParam uintptr) uintptr {
	if code >= 0 && wParam == wmKeydown {
		if readKbdLLVkCode(lParam) == winHookKeyCode && winHookDetector != nil {
			winHookDetector.Press()
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return ret
}

func startWindowsHook(openKey string, intervalMs int, fn func()) {
	winHookKeyCode = vkCodeMap[openKey]
	winHookDetector = &DoublePressDetector{
		fn:       fn,
		interval: time.Duration(intervalMs) * time.Millisecond,
	}
	hookCallback = syscall.NewCallback(winLowLevelKeyboardProc)

	hook, _, _ := procSetWindowsHookEx.Call(whKeyboardLL, hookCallback, 0, 0)
	if hook == 0 {
		return
	}
	hookHandle = hook

	for {
		var m msg
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 || ret == 0xFFFFFFFF {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	procUnhookWindowsHookEx.Call(hookHandle)
}

func startLinuxHook(openKey string, intervalMs int, fn func()) {}

// Apply reaplica a configuração de hotkey em runtime (Windows).
func Apply(openKey string, intervalMs int) {
	winHookKeyCode = vkCodeMap[openKey]
	if winHookDetector != nil {
		winHookDetector.interval = time.Duration(intervalMs) * time.Millisecond
	}
}
