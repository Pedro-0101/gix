package ui

import (
	"gix/internal/config"
	"gix/internal/db"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

var (
	a             fyne.App
	w             fyne.Window
	entry         *escEntry
	settingsBtn   *widget.Button
	saveBtn       *widget.Button
	notesList     *widget.List
	desk          desktop.App
	currentConfig *config.Config
	configMu      sync.RWMutex
	database      *db.Database
	notes         []db.Note
	notesMu       sync.Mutex
)

var fyneKeyNameMap = map[string]fyne.KeyName{
	"Space":  fyne.KeySpace,
	"Escape": fyne.KeyEscape,
	"Tab":    fyne.KeyTab,
	"Enter":  fyne.KeyReturn,
}

func getConfig() *config.Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return currentConfig
}

func setConfig(c *config.Config) {
	configMu.Lock()
	defer configMu.Unlock()
	currentConfig = c
}

func removeButtons(hwnd uintptr) {
	style, _, _ := getWindowLongW.Call(hwnd, gwlStyle)
	style &^= uintptr(wsMinimizeBox | wsMaximizeBox | wsSysMenu)
	setWindowLongW.Call(hwnd, gwlStyle, style)
	setWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)
}

type escEntry struct {
	widget.Entry
	onDoubleKey func()
	count       int
	keyName     fyne.KeyName
	interval    int
}

func (e *escEntry) TypedKey(k *fyne.KeyEvent) {
	if k.Name == e.keyName {
		e.count++
		if e.count >= 2 {
			e.count = 0
			if e.onDoubleKey != nil {
				e.onDoubleKey()
			}
			return
		}
		time.AfterFunc(time.Duration(e.interval)*time.Millisecond, func() {
			e.count = 0
		})
		return
	}
	e.count = 0
	e.Entry.TypedKey(k)
}

func loadNotes() {
	if database == nil {
		return
	}
	list, err := database.List()
	if err != nil {
		return
	}
	notesMu.Lock()
	notes = list
	notesMu.Unlock()
	if notesList != nil {
		notesList.Refresh()
	}
}

func saveNote() {
	text := entry.Text
	if strings.TrimSpace(text) == "" {
		return
	}
	if database == nil {
		return
	}
	title := db.ExtractTitle(text)
	_, err := database.Create(title, text)
	if err != nil {
		return
	}
	entry.SetText("")
	loadNotes()
}

func deleteNote(id int64) {
	if database == nil {
		return
	}
	_ = database.Delete(id)
	loadNotes()
}

func Run() {
	cfg := config.Load()
	setConfig(cfg)

	var err error
	database, err = db.New()
	if err != nil {
		database = nil
	}

	a = app.New()

	if cfg.Theme == "dark" {
		a.Settings().SetTheme(theme.DarkTheme())
	}

	w = a.NewWindow("gix")

	w.SetFixedSize(true)
	w.Resize(fyne.NewSize(400, 500))

	closeKey := fyne.KeyEscape
	if key, ok := fyneKeyNameMap[cfg.CloseKey]; ok {
		closeKey = key
	}

	entry = &escEntry{
		onDoubleKey: func() {
			w.Hide()
		},
		keyName:  closeKey,
		interval: cfg.CloseIntervalMs,
	}
	entry.ExtendBaseWidget(entry)
	entry.PlaceHolder = getTr("placeholder")

	closeDetector := &doublePressDetector{
		fn: func() {
			w.Hide()
		},
		interval: time.Duration(cfg.CloseIntervalMs) * time.Millisecond,
	}
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == closeKey {
			closeDetector.press()
		}
	})

	settingsBtn = widget.NewButton(getTr("settings"), func() {
		showSettingsWindow(a, w)
	})

	saveBtn = widget.NewButton(getTr("save_note"), func() {
		saveNote()
	})

	notes = []db.Note{}
	if database != nil {
		notes, _ = database.List()
	}

	notesList = widget.NewList(
		func() int {
			notesMu.Lock()
			n := len(notes)
			notesMu.Unlock()
			return n
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
			delBtn.Importance = widget.DangerImportance
			return container.NewBorder(nil, nil, nil, delBtn, label)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			notesMu.Lock()
			if id >= len(notes) {
				notesMu.Unlock()
				return
			}
			n := notes[id]
			notesMu.Unlock()
			c := item.(*fyne.Container)
			label := c.Objects[0].(*widget.Label)
			delBtn := c.Objects[1].(*widget.Button)
			label.SetText(n.Title)
			delBtn.OnTapped = func() {
				deleteNote(n.ID)
			}
		},
	)

	header := container.NewHBox(layout.NewSpacer(), settingsBtn)
	content := container.NewBorder(
		header,
		nil,
		nil,
		nil,
		container.NewVBox(entry, saveBtn, notesList),
	)
	w.SetContent(content)

	if d, ok := a.(desktop.App); ok {
		desk = d
		rebuildTrayMenu()
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
		fyne.Do(func() {
			w.Show()
			w.RequestFocus()
			w.Canvas().Focus(entry)
		})
	}, cfg)

	w.ShowAndRun()

	if database != nil {
		database.Close()
	}
}
