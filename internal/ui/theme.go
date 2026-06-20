package ui

import (
	"image/color"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
)

var gixThemeInstance *gixTheme

type gixTheme struct {
	base       fyne.Theme
	regular    fyne.Resource
	bold       fyne.Resource
	italic     fyne.Resource
	boldItalic fyne.Resource
}

func (t *gixTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return t.base.Color(name, variant)
}

func (t *gixTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *gixTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}

func (t *gixTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Bold && style.Italic {
		if t.boldItalic != nil {
			return t.boldItalic
		}
	}
	if style.Bold {
		if t.bold != nil {
			return t.bold
		}
	}
	if style.Italic {
		if t.italic != nil {
			return t.italic
		}
	}
	if t.regular != nil {
		return t.regular
	}
	return t.base.Font(style)
}

func loadConsolasFonts() (regular, bold, italic, boldItalic fyne.Resource) {
	fontDir := ""
	switch runtime.GOOS {
	case "windows":
		if root := os.Getenv("SystemRoot"); root != "" {
			fontDir = filepath.Join(root, "Fonts")
		}
	case "linux":
		candidates := []string{
			"/usr/share/fonts",
			"/usr/local/share/fonts",
			filepath.Join(os.Getenv("HOME"), ".fonts"),
		}
		for _, d := range candidates {
			if info, err := os.Stat(d); err == nil && info.IsDir() {
				fontDir = d
				break
			}
		}
	case "darwin":
		fontDir = "/Library/Fonts"
	}

	if fontDir == "" {
		return
	}

	load := func(name string) fyne.Resource {
		data, err := os.ReadFile(filepath.Join(fontDir, name))
		if err != nil {
			return nil
		}
		return fyne.NewStaticResource(name, data)
	}

	regular = load("consola.ttf")
	bold = load("consolab.ttf")
	italic = load("consolai.ttf")
	boldItalic = load("consolaz.ttf")
	return
}

func newGixTheme(base fyne.Theme) *gixTheme {
	r, b, i, bi := loadConsolasFonts()
	return &gixTheme{
		base:       base,
		regular:    r,
		bold:       b,
		italic:     i,
		boldItalic: bi,
	}
}

func applyTheme(base fyne.Theme) {
	if gixThemeInstance == nil {
		gixThemeInstance = newGixTheme(base)
	} else {
		gixThemeInstance.base = base
	}
	a.Settings().SetTheme(gixThemeInstance)
}
