package ui

import (
	"strconv"
	"sync"

	"gix/internal/config"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	ptTranslations = map[string]string{
		"settings":       "Configurações",
		"theme":          "Tema",
		"light":          "Claro",
		"dark":           "Escuro",
		"language":       "Idioma",
		"portuguese":     "Português",
		"english":        "Inglês",
		"open_hotkey":    "Tecla para abrir",
		"open_interval":  "Intervalo para abrir (ms)",
		"close_hotkey":   "Tecla para fechar",
		"close_interval": "Intervalo para fechar (ms)",
		"model":          "Modelo",
		"api_key":        "Chave da API",
		"system_prompt":  "Prompt de sistema",
		"save":           "Salvar",
		"cancel":         "Cancelar",
		"placeholder":    "pergunte algo…",
		"show":           "Exibir",
		"quit":           "Sair",
		"history":        "Histórico",
		"history_empty":  "Nenhuma conversa salva.",
		"you":            "Você",
		"ai":             "IA",
		"thinking":       "pensando…",
		"no_api_key":     "Configure a chave do OpenRouter nas configurações.",
		"error_prefix":   "Erro: ",
		"empty_response": "(sem resposta)",
		"copy":           "Copiar",
	}
	enTranslations = map[string]string{
		"settings":       "Settings",
		"theme":          "Theme",
		"light":          "Light",
		"dark":           "Dark",
		"language":       "Language",
		"portuguese":     "Portuguese",
		"english":        "English",
		"open_hotkey":    "Open hotkey",
		"open_interval":  "Open interval (ms)",
		"close_hotkey":   "Close hotkey",
		"close_interval": "Close interval (ms)",
		"model":          "Model",
		"api_key":        "API key",
		"system_prompt":  "System prompt",
		"save":           "Save",
		"cancel":         "Cancel",
		"placeholder":    "ask something…",
		"show":           "Show",
		"quit":           "Quit",
		"history":        "History",
		"history_empty":  "No saved conversations.",
		"you":            "You",
		"ai":             "AI",
		"thinking":       "thinking…",
		"no_api_key":     "Set your OpenRouter key in settings.",
		"error_prefix":   "Error: ",
		"empty_response": "(no response)",
		"copy":           "Copy",
	}
	trMu sync.RWMutex
)

func getTr(key string) string {
	cfg := getConfig()
	m := ptTranslations
	if cfg.Language == "en" {
		m = enTranslations
	}
	if v, ok := m[key]; ok {
		return v
	}
	return key
}

var (
	keyOptions = []string{"Space", "Escape", "Tab", "Enter"}
)

func showSettingsWindow(a fyne.App, parent fyne.Window) {
	cfg := getConfig()

	sw := a.NewWindow(getTr("settings"))

	themeRadio := widget.NewRadioGroup([]string{getTr("light"), getTr("dark")}, nil)
	langRadio := widget.NewRadioGroup([]string{getTr("portuguese"), getTr("english")}, nil)
	openKeySelect := widget.NewSelect(keyOptions, nil)
	openIntervalEntry := widget.NewEntry()
	closeKeySelect := widget.NewSelect(keyOptions, nil)
	closeIntervalEntry := widget.NewEntry()
	modelSelect := widget.NewSelect(config.Models, nil)
	apiKeyEntry := widget.NewPasswordEntry()
	systemPromptEntry := widget.NewMultiLineEntry()

	if cfg.Theme == "dark" {
		themeRadio.SetSelected(getTr("dark"))
	} else {
		themeRadio.SetSelected(getTr("light"))
	}
	if cfg.Language == "en" {
		langRadio.SetSelected(getTr("english"))
	} else {
		langRadio.SetSelected(getTr("portuguese"))
	}
	openKeySelect.SetSelected(cfg.OpenKey)
	openIntervalEntry.SetText(strconv.Itoa(cfg.OpenIntervalMs))
	closeKeySelect.SetSelected(cfg.CloseKey)
	closeIntervalEntry.SetText(strconv.Itoa(cfg.CloseIntervalMs))
	modelSelect.SetSelected(cfg.Model)
	apiKeyEntry.SetText(cfg.APIKey)
	systemPromptEntry.SetText(cfg.SystemPrompt)

	form := widget.NewForm(
		&widget.FormItem{Text: getTr("theme"), Widget: themeRadio},
		&widget.FormItem{Text: getTr("language"), Widget: langRadio},
		&widget.FormItem{Text: "", Widget: widget.NewSeparator()},
		&widget.FormItem{Text: getTr("model"), Widget: modelSelect},
		&widget.FormItem{Text: getTr("api_key"), Widget: apiKeyEntry},
		&widget.FormItem{Text: getTr("system_prompt"), Widget: systemPromptEntry},
		&widget.FormItem{Text: "", Widget: widget.NewSeparator()},
		&widget.FormItem{Text: getTr("open_hotkey"), Widget: openKeySelect},
		&widget.FormItem{Text: getTr("open_interval"), Widget: openIntervalEntry},
		&widget.FormItem{Text: "", Widget: widget.NewSeparator()},
		&widget.FormItem{Text: getTr("close_hotkey"), Widget: closeKeySelect},
		&widget.FormItem{Text: getTr("close_interval"), Widget: closeIntervalEntry},
	)

	form.SubmitText = getTr("save")
	form.CancelText = getTr("cancel")
	form.OnSubmit = func() {
		newCfg := *cfg

		if themeRadio.Selected == getTr("dark") {
			newCfg.Theme = "dark"
		} else {
			newCfg.Theme = "light"
		}
		if langRadio.Selected == getTr("english") {
			newCfg.Language = "en"
		} else {
			newCfg.Language = "pt"
		}
		newCfg.OpenKey = openKeySelect.Selected
		newCfg.CloseKey = closeKeySelect.Selected
		if interval, err := strconv.Atoi(openIntervalEntry.Text); err == nil && interval > 0 {
			newCfg.OpenIntervalMs = interval
		}
		if interval, err := strconv.Atoi(closeIntervalEntry.Text); err == nil && interval > 0 {
			newCfg.CloseIntervalMs = interval
		}
		newCfg.Model = modelSelect.Selected
		newCfg.APIKey = apiKeyEntry.Text
		newCfg.SystemPrompt = systemPromptEntry.Text

		if newCfg.Theme == "dark" {
			a.Settings().SetTheme(theme.DarkTheme())
		} else {
			a.Settings().SetTheme(theme.LightTheme())
		}

		newCfg.Save()
		setConfig(&newCfg)

		if entry != nil {
			if key, ok := fyneKeyNameMap[newCfg.CloseKey]; ok {
				entry.keyName = key
			}
			entry.interval = newCfg.CloseIntervalMs
			entry.PlaceHolder = getTr("placeholder")
			entry.Refresh()
		}

		rebuildTrayMenu()
		applyHotkeyConfig(&newCfg)

		sw.Close()
	}
	form.OnCancel = func() {
		sw.Close()
	}

	content := container.NewPadded(form)
	sw.SetContent(content)
	sw.Resize(fyne.NewSize(420, 620))
	sw.CenterOnScreen()
	sw.Show()
}

func rebuildTrayMenu() {
	if desk == nil {
		return
	}
	m := fyne.NewMenu("gix",
		fyne.NewMenuItem(getTr("show"), func() {
			if w != nil {
				fyne.Do(showWindow)
			}
		}),
		fyne.NewMenuItem(getTr("quit"), func() {
			if w != nil {
				w.Close()
			}
			if a != nil {
				a.Quit()
			}
		}),
	)
	desk.SetSystemTrayMenu(m)
}
