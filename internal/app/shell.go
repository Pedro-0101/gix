// Package app monta o shell desktop do gix (overlay Wails: janela frameless,
// tray, hotkey global, config local e serviço de notificações toast).
//
// Toda a lógica de backend (chat/notas/alertas/histórico/IA/embeddings/scheduler)
// vive no gix-server e é acessada pelo frontend via HTTP/SSE. O Go daqui só
// cuida do que é intrinsecamente desktop: janela, tray, hotkey, prefs locais e
// o serviço de notificações nativas (chamado pelo frontend quando um push SSE
// de alerta chega).
package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"gix/internal/config"
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

// Run bootstraps the Wails v3 application: registers the services, creates the
// frameless always-on-top window (hidden at boot), the system tray, and the
// global hotkey that shows/centers/focuses the window.
func Run(assets fs.FS, trayIcon []byte) error {
	cfgSvc := NewConfigService()

	var wailsApp *application.App
	emit := func(name string, data any) {
		if wailsApp != nil {
			wailsApp.Event.Emit(name, data)
		}
	}

	// Serviço de notificações nativas: o frontend o chama (via binding) quando
	// um push SSE de alerta chega do gix-server, para erguer o toast do SO.
	notifSvc := notifications.New()

	// Cofre de sessão: persiste o par de tokens JWT cifrado (DPAPI no Windows),
	// para o frontend não guardar segredo em localStorage.
	tokenSvc := NewTokenService()

	wailsApp = application.New(application.Options{
		Name:        "gix",
		Description: "gix — overlay de chat com IA",
		Services: []application.Service{
			application.NewService(cfgSvc),
			application.NewService(notifSvc),
			application.NewService(tokenSvc),
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
	// cursor currently sat. Reset to the collapsed height first so the window
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

	// Closing the window hides it instead of quitting. O cancelamento de stream
	// de chat é feito pelo frontend (AbortController) — o Go só esconde a janela.
	mainWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
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

	// Garantia de diretório de dados do usuário (não usado diretamente aqui, mas
	// mantido para futuros caches locais do desktop). Sem erro => segue.
	if dir, err := os.UserConfigDir(); err == nil {
		_ = os.MkdirAll(filepath.Join(dir, "gix"), 0o755)
	}

	err := wailsApp.Run()
	return err
}
