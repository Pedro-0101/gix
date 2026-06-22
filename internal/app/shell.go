package app

import (
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"gix/internal/ai"
	"gix/internal/config"
	"gix/internal/db"
	"gix/internal/hotkey"
)

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
		func(apiKey string) Streamer { return ai.New(apiKey) })

	wailsApp = application.New(application.Options{
		Name:        "gix",
		Description: "gix — overlay de chat com IA",
		Services: []application.Service{
			application.NewService(cfgSvc),
			application.NewService(histSvc),
			application.NewService(chatSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	mainWin := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "gix",
		Width:         640,
		Height:        480,
		Frameless:     true,
		AlwaysOnTop:   true,
		Hidden:        true,
		DisableResize: true,
		URL:           "/",
	})

	showMain := func() {
		mainWin.Show()
		mainWin.Center()
		mainWin.Focus()
	}

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

	// Global hotkey (double-press of the configured open key) shows the window.
	cur := cfgSvc.Current()
	hotkey.Start(cur.OpenKey, cur.OpenIntervalMs, func() { showMain() })
	cfgSvc.OnSave(func(c *config.Config) {
		hotkey.Apply(c.OpenKey, c.OpenIntervalMs)
	})

	err = wailsApp.Run()

	if database != nil {
		database.Close()
	}
	return err
}
