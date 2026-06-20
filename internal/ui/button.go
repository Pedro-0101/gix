package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// cursorButton is a widget.Button that shows the pointer (hand) cursor while
// hovered, signalling clickability like a web button.
type cursorButton struct {
	widget.Button
}

func newIconButton(icon fyne.Resource, tapped func()) *cursorButton {
	b := &cursorButton{}
	b.Icon = icon
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

// Cursor implements desktop.Cursorable.
func (b *cursorButton) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}
