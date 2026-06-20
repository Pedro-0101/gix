package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	cardMinWidthFrac = 0.30
	cardMaxWidthFrac = 0.75
)

type chatCard struct {
	widget.BaseWidget
	body   *widget.Label
	isUser bool
}

func newChatCard(text string, isUser bool) *chatCard {
	c := &chatCard{
		body:   widget.NewLabel(text),
		isUser: isUser,
	}
	c.body.Wrapping = fyne.TextWrapWord
	c.ExtendBaseWidget(c)
	return c
}

func (c *chatCard) CreateRenderer() fyne.WidgetRenderer {
	role := getTr("ai")
	if c.isUser {
		role = getTr("you")
	}
	roleLabel := widget.NewLabelWithStyle(role, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(c.body.Text)
	})
	copyBtn.Importance = widget.LowImportance

	header := container.NewHBox(roleLabel, layout.NewSpacer(), copyBtn)
	inner := container.NewBorder(header, nil, nil, nil, c.body)

	bg := canvas.NewRectangle(theme.InputBackgroundColor())
	bg.CornerRadius = 12
	bg.StrokeWidth = 0

	padded := container.NewPadded(inner)
	bubble := container.NewStack(bg, padded)

	return &chatCardRenderer{
		card:    c,
		bg:      bg,
		inner:   inner,
		bubble:  bubble,
		role:    roleLabel,
		copyBtn: copyBtn,
	}
}

type chatCardRenderer struct {
	card    *chatCard
	bg      *canvas.Rectangle
	inner   *fyne.Container
	bubble  *fyne.Container
	role    *widget.Label
	copyBtn *widget.Button
}

func (r *chatCardRenderer) Layout(size fyne.Size) {
	minW := size.Width * cardMinWidthFrac
	maxW := size.Width * cardMaxWidthFrac
	bubbleMinW := r.bubble.MinSize().Width
	bubbleW := max(minW, min(maxW, bubbleMinW))

	if r.card.isUser {
		r.bubble.Resize(fyne.NewSize(bubbleW, size.Height))
		r.bubble.Move(fyne.NewPos(size.Width-bubbleW, 0))
	} else {
		r.bubble.Resize(fyne.NewSize(bubbleW, size.Height))
		r.bubble.Move(fyne.NewPos(0, 0))
	}
}

func (r *chatCardRenderer) MinSize() fyne.Size {
	return r.bubble.MinSize()
}

func (r *chatCardRenderer) Refresh() {
	if r.card.isUser {
		r.bg.FillColor = theme.PrimaryColor()
	} else {
		r.bg.FillColor = theme.InputBackgroundColor()
	}
	canvas.Refresh(r.bg)
}

func (r *chatCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bubble}
}

func (r *chatCardRenderer) Destroy() {}
