package ui

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	findWindowW    = user32.NewProc("FindWindowW")
	getWindowLongW = user32.NewProc("GetWindowLongW")
	setWindowLongW = user32.NewProc("SetWindowLongW")
	setWindowPos   = user32.NewProc("SetWindowPos")
)

const (
	gwlStyle      = ^uintptr(15)
	wsMinimizeBox = 0x00020000
	wsMaximizeBox = 0x00010000
	wsSysMenu     = 0x00080000

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpFrameChanged = 0x0020
)

func removeButtons(hwnd uintptr) {
	style, _, _ := getWindowLongW.Call(hwnd, gwlStyle)
	style &^= uintptr(wsMinimizeBox | wsMaximizeBox | wsSysMenu)
	setWindowLongW.Call(hwnd, gwlStyle, style)
	setWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)
}

type escEntry struct {
	widget.Entry
	onDoubleEsc func()
	count       int
}

func (e *escEntry) TypedKey(k *fyne.KeyEvent) {
	if k.Name == fyne.KeyEscape {
		e.count++
		if e.count >= 2 {
			e.count = 0
			if e.onDoubleEsc != nil {
				e.onDoubleEsc()
			}
			return
		}
		time.AfterFunc(500*time.Millisecond, func() {
			e.count = 0
		})
		return
	}
	e.count = 0
	e.Entry.TypedKey(k)
}

func Run() {
	a := app.New()
	w := a.NewWindow("gix")

	w.SetFixedSize(true)
	w.Resize(fyne.NewSize(400, 300))

	entry := &escEntry{onDoubleEsc: func() {
		w.Hide()
	}}
	entry.ExtendBaseWidget(entry)
	entry.PlaceHolder = "Digite algo..."

	w.SetContent(entry)

	if desk, ok := a.(desktop.App); ok {
		m := fyne.NewMenu("gix",
			fyne.NewMenuItem("Exibir", func() {
				w.Show()
				w.RequestFocus()
			}),
			fyne.NewMenuItem("Sair", func() {
				w.Close()
				a.Quit()
			}),
		)
		desk.SetSystemTrayMenu(m)
	}

	if runtime.GOOS == "windows" {
		go func() {
			time.Sleep(200 * time.Millisecond)
			titlePtr, _ := syscall.UTF16PtrFromString("gix")
			hwnd, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
			if hwnd != 0 {
				removeButtons(hwnd)
			}
		}()
	}

	startHotkeyListener(func() {
		w.Show()
		w.RequestFocus()
	})

	w.ShowAndRun()
}
