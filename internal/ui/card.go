package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

func newChatCard(content fyne.CanvasObject) *fyne.Container {
	bg := canvas.NewRectangle(theme.InputBackgroundColor())
	bg.StrokeColor = theme.ShadowColor()
	bg.StrokeWidth = 1

	padded := container.NewPadded(content)
	return container.NewStack(bg, padded)
}
