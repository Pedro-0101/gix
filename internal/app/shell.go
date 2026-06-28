package app

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"gix/internal/config"
	"gix/internal/db"
	"gix/internal/embed"
	"gix/internal/hotkey"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procGetCursorPos = user32.NewProc("GetCursorPos")
)

type _point struct {
	X, Y int32
}

// cursorScreen returns the Screen that currently contains the mouse cursor.
// Unlike mainWin.GetScreen() (which returns the screen the hidden window
// happens to sit on, usually monitor 1), this picks the display the user is
// actually working on — the same heuristic Spotlight/Raycast use.
func cursorScreen(screens []*application.Screen) *application.Screen {
	var pt _point
	ret, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return nil
	}
	for _, s := range screens {
		b := s.PhysicalBounds
		if int(pt.X) >= b.X && int(pt.X) < b.X+b.Width &&
			int(pt.Y) >= b.Y && int(pt.Y) < b.Y+b.Height {
			return s
		}
	}
	return nil
}

// modelsDir is where the embedding model/tokenizer are cached, under the same
// per-user config dir as the database (e.g. %AppData%/gix/models).
func modelsDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "models"
	}
	return filepath.Join(dir, "gix", "models")
}

// Run bootstraps the Wails v3 application: registers the services, creates the
// frameless always-on-top window (hidden at boot), the system tray, and the
// global hotkey that shows/centers/focuses the window.
func Run(assets fs.FS, trayIcon []byte) error {
	database, err := db.New()
	if err != nil {
		database = nil
	}

	cfgSvc := NewConfigService()
	histSvc := NewHistoryService(database)

	var wailsApp *application.App
	emit := func(name string, data any) {
		if wailsApp != nil {
			wailsApp.Event.Emit(name, data)
		}
	}
	chatSvc := NewChatService(cfgSvc, database, emit,
		func(apiKey string) Streamer { return newProvider(apiKey) })
	notesSvc := NewNotesService(cfgSvc, database,
		func(apiKey string) Completer { return newProvider(apiKey) })

	notifSvc := notifications.New()

	// showMain is defined below (it needs mainWin). The alerts scheduler calls it
	// through this indirection so a fired alert can surface the overlay.
	var showMainFn func()
	onShow := func() {
		if showMainFn != nil {
			showMainFn()
		}
	}
	alertsSvc := NewAlertsService(cfgSvc, database,
		func(apiKey string) Completer { return newProvider(apiKey) },
		emit, onShow, notifSvc)

	// Load the embedding model in the background so the UI starts instantly. The
	// model+tokenizer download on first run (progress is forwarded to the
	// frontend); until it's ready, search falls back to full-text only.
	embed.OnDownloadProgress = func(file string, downloaded, total int64) {
		emit("embed:progress", map[string]any{"file": file, "downloaded": downloaded, "total": total})
	}
	go func() {
		e, err := embed.Open(context.Background(), modelsDir())
		if err != nil {
			log.Printf("embed: model unavailable, semantic search disabled: %v", err)
			emit("embed:error", err.Error())
			return
		}
		notesSvc.setEmbedder(e)
		emit("embed:ready", nil)
	}()

	wailsApp = application.New(application.Options{
		Name:        "gix",
		Description: "gix — overlay de chat com IA",
		Services: []application.Service{
			application.NewService(cfgSvc),
			application.NewService(histSvc),
			application.NewService(chatSvc),
			application.NewService(notesSvc),
			application.NewService(notifSvc),
			application.NewService(alertsSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	// Command-palette dimensions. The window opens collapsed (just the input
	// bar) at the top-centre of the screen; the frontend grows the height as
	// answers stream in, anchored to this top edge so it expands downward.
	const (
		paletteWidth    = 680
		collapsedHeight = 64
		topOffset       = 120
	)

	mainWin := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:       "gix",
		Width:       paletteWidth,
		Height:      collapsedHeight,
		Frameless:   true,
		AlwaysOnTop: true,
		Hidden:      true,
		// The palette sizes itself to its content; the user must not be able to
		// drag the edges to resize it.
		DisableResize: true,
		// Frosted-glass overlay: the OS composites an Acrylic backdrop behind the
		// window and the web content paints transparently on top (no CSS shell
		// background). Acrylic is the reliable native translucency path on Windows;
		// BackgroundTypeTransparent renders opaque white on this WebView2/Wails build.
		BackgroundType: application.BackgroundTypeTranslucent,
		URL:            "/",
		Windows: application.WindowsWindow{
			BackdropType: application.Acrylic,
		},
	})

	// Show the palette pinned to the top-centre of the monitor where the mouse
	// cursor currently sits. Reset to the collapsed height first so the window
	// always re-opens as the bare input bar and grows downward as content
	// streams in (the frontend animates the height via useWindowFit). Without
	// this the OS keeps the previous session's height, so re-opening to a clean
	// bar would flash the old, taller window and visibly shrink.
	showMain := func() {
		s := cursorScreen(wailsApp.Screen.GetAll())
		if s == nil {
			s, _ = mainWin.GetScreen()
		}
		mainWin.SetSize(paletteWidth, collapsedHeight)
		if s != nil {
			wa := s.WorkArea
			mainWin.SetPosition(wa.X+(wa.Width-paletteWidth)/2, wa.Y+topOffset)
		} else {
			mainWin.Center()
		}
		mainWin.Show()
		mainWin.Focus()
		emit("window:shown", nil)
	}

	showMainFn = showMain

	// Toast action buttons + click handling. The buttons and their handlers live
	// together in the alerts service (alerts_actions.go); shell.go only wires the
	// service's dispatcher to the notifier.
	alertsSvc.RegisterCategory()
	notifSvc.OnNotificationResponse(alertsSvc.HandleNotificationResponse)

	// Scheduler goroutine; cancelled on app shutdown.
	alertCtx, cancelAlerts := context.WithCancel(context.Background())
	go alertsSvc.Run(alertCtx)

	// Closing the window hides it (and cancels any in-flight stream) instead of quitting.
	mainWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		chatSvc.Cancel()
		mainWin.Hide()
		e.Cancel()
	})

	tray := wailsApp.SystemTray.New()
	tray.SetIcon(trayIcon)
	menu := wailsApp.NewMenu()
	menu.Add("Exibir").OnClick(func(_ *application.Context) { showMain() })
	menu.Add("Sair").OnClick(func(_ *application.Context) { wailsApp.Quit() })
	tray.SetMenu(menu)

	// Global hotkey (N presses of the configured open key) shows the window.
	cur := cfgSvc.Current()
	hotkey.Start(cur.OpenKey, cur.OpenIntervalMs, cur.OpenPressCount, func() { showMain() })
	cfgSvc.OnSave(func(c *config.Config) {
		hotkey.Apply(c.OpenKey, c.OpenIntervalMs, c.OpenPressCount)
	})

	err = wailsApp.Run()

	cancelAlerts()
	if database != nil {
		database.Close()
	}
	return err
}
