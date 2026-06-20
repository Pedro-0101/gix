package ui

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"gix/internal/ai"
	"gix/internal/config"
	"gix/internal/db"

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
	a              fyne.App
	w              fyne.Window
	entry          *escEntry
	settingsBtn    *widget.Button
	historyBtn     *widget.Button
	messagesBox    *fyne.Container
	messagesScroll *container.Scroll
	desk           desktop.App
	currentConfig  *config.Config
	configMu       sync.RWMutex
	database       *db.Database

	chatMu     sync.Mutex
	convID     int64
	history    []ai.Message
	streaming  bool
	cancelFunc context.CancelFunc
	convGen    uint64
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
	shiftDown   bool
}

func (e *escEntry) KeyDown(k *fyne.KeyEvent) {
	if k.Name == desktop.KeyShiftLeft || k.Name == desktop.KeyShiftRight {
		e.shiftDown = true
	}
	e.Entry.KeyDown(k)
}

func (e *escEntry) KeyUp(k *fyne.KeyEvent) {
	if k.Name == desktop.KeyShiftLeft || k.Name == desktop.KeyShiftRight {
		e.shiftDown = false
	}
	e.Entry.KeyUp(k)
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

	if k.Name == fyne.KeyReturn || k.Name == fyne.KeyEnter {
		if e.shiftDown {
			e.Entry.TypedKey(k)
		} else {
			sendMessage()
		}
		return
	}

	e.count = 0
	e.Entry.TypedKey(k)
}

// appendMessage adiciona um bloco de mensagem na área de chat e devolve o
// entry do corpo (para o streaming continuar atualizando). Roda na UI.
func appendMessage(roleLabel, text string) *widget.Entry {
	prefix := widget.NewLabelWithStyle(roleLabel, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	body := widget.NewEntry()
	body.SetText(text)
	body.MultiLine = true
	body.Wrapping = fyne.TextWrapWord
	body.Disable()

	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(body.Text)
	})
	copyBtn.Importance = widget.LowImportance

	header := container.NewHBox(prefix, layout.NewSpacer(), copyBtn)
	card := widget.NewCard("", "", container.NewVBox(header, body))
	messagesBox.Add(card)
	messagesScroll.ScrollToBottom()
	return body
}

func newConversation() {
	chatMu.Lock()
	convID = 0
	history = nil
	convGen++
	chatMu.Unlock()
	if messagesBox != nil {
		messagesBox.RemoveAll()
		messagesBox.Refresh()
	}
}

func maybeNewConversation() {
	chatMu.Lock()
	has := len(history) > 0
	chatMu.Unlock()
	if has {
		newConversation()
	}
}

func showWindow() {
	maybeNewConversation()
	w.Show()
	w.RequestFocus()
	w.Canvas().Focus(entry)
}

func hideWindow() {
	chatMu.Lock()
	cancel := cancelFunc
	chatMu.Unlock()
	if cancel != nil {
		cancel()
	}
	w.Hide()
}

func sendMessage() {
	text := strings.TrimSpace(entry.Text)
	if text == "" {
		return
	}

	chatMu.Lock()
	isStreaming := streaming
	chatMu.Unlock()
	if isStreaming {
		return
	}

	cfg := getConfig()
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		appendMessage(getTr("ai"), getTr("no_api_key"))
		entry.SetText("")
		return
	}

	entry.SetText("")
	appendMessage(getTr("you"), text)

	chatMu.Lock()
	if convID == 0 && database != nil {
		if id, err := database.CreateConversation(db.ExtractTitle(text), cfg.Model); err == nil {
			convID = id
		}
	}
	cid := convID
	history = append(history, ai.Message{Role: "user", Content: text})
	msgs := make([]ai.Message, 0, len(history)+1)
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		msgs = append(msgs, ai.Message{Role: "system", Content: cfg.SystemPrompt})
	}
	msgs = append(msgs, history...)
	streaming = true
	gen := convGen
	ctx, cancel := context.WithCancel(context.Background())
	cancelFunc = cancel
	chatMu.Unlock()

	if database != nil && cid != 0 {
		_ = database.AddMessage(cid, "user", text)
	}

	label := appendMessage(getTr("ai"), getTr("thinking"))

	go func() {
		client := ai.New(apiKey)
		var sb strings.Builder
		streamErr := client.Stream(ctx, cfg.Model, msgs, func(delta string) {
			sb.WriteString(delta)
			current := sb.String()
			fyne.Do(func() {
				label.SetText(current)
				messagesScroll.ScrollToBottom()
			})
		})
		full := sb.String()

		chatMu.Lock()
		streaming = false
		cancelFunc = nil
		chatMu.Unlock()

		switch {
		case errors.Is(streamErr, context.Canceled):
			if full != "" {
				if database != nil && cid != 0 {
					_ = database.AddMessage(cid, "assistant", full)
				}
				chatMu.Lock()
				if convGen == gen {
					history = append(history, ai.Message{Role: "assistant", Content: full})
				}
				chatMu.Unlock()
			}
		case streamErr != nil:
			msg := getTr("error_prefix") + streamErr.Error()
			fyne.Do(func() {
				label.SetText(msg)
				messagesScroll.ScrollToBottom()
			})
		default:
			if full == "" {
				full = getTr("empty_response")
				fyne.Do(func() { label.SetText(full) })
			}
			if database != nil && cid != 0 {
				_ = database.AddMessage(cid, "assistant", full)
			}
			chatMu.Lock()
			if convGen == gen {
				history = append(history, ai.Message{Role: "assistant", Content: full})
			}
			chatMu.Unlock()
		}
	}()
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
	w.Resize(fyne.NewSize(400, 600))

	closeKey := fyne.KeyEscape
	if key, ok := fyneKeyNameMap[cfg.CloseKey]; ok {
		closeKey = key
	}

	entry = &escEntry{
		onDoubleKey: hideWindow,
		keyName:     closeKey,
		interval:    cfg.CloseIntervalMs,
	}
	entry.ExtendBaseWidget(entry)
	entry.PlaceHolder = getTr("placeholder")
	entry.MultiLine = true

	closeDetector := &doublePressDetector{
		fn:       hideWindow,
		interval: time.Duration(cfg.CloseIntervalMs) * time.Millisecond,
	}
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == closeKey {
			closeDetector.press()
		}
	})

	settingsBtn = widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		showSettingsWindow(a, w)
	})
	historyBtn = widget.NewButtonWithIcon("", theme.HistoryIcon(), func() {
		showHistoryWindow(a)
	})

	messagesBox = container.NewVBox()
	messagesScroll = container.NewVScroll(messagesBox)

	header := container.NewHBox(layout.NewSpacer(), historyBtn, settingsBtn)
	content := container.NewBorder(header, entry, nil, nil, messagesScroll)
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
		fyne.Do(showWindow)
	}, cfg)

	w.ShowAndRun()

	if database != nil {
		database.Close()
	}
}
