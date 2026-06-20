package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const cardMaxWidthFrac = 0.75

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

func (c *chatCard) bodyText() string {
	return c.body.Text
}

func (c *chatCard) CreateRenderer() fyne.WidgetRenderer {
	roleLabel := widget.NewLabelWithStyle(getTr("ai"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	if c.isUser {
		roleLabel.SetText(getTr("you"))
	}

	copyBtn := newIconButton(theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(c.bodyText())
	})
	copyBtn.Importance = widget.LowImportance

	header := container.NewHBox(roleLabel, layout.NewSpacer(), copyBtn)

	bg := canvas.NewRectangle(theme.InputBackgroundColor())
	bg.CornerRadius = 12
	bg.StrokeWidth = 0

	bodyPadded := container.NewPadded(c.body)
	bodyBubble := container.NewStack(bg, bodyPadded)

	content := container.NewVBox(header, bodyBubble)

	return &chatCardRenderer{
		card:   c,
		bg:     bg,
		body:   bodyBubble,
		header: header,
		content: content,
	}
}

type chatCardRenderer struct {
	card    *chatCard
	bg      *canvas.Rectangle
	body    *fyne.Container
	header  *fyne.Container
	content *fyne.Container
}

func (r *chatCardRenderer) Layout(size fyne.Size) {
	bubbleW := size.Width * cardMaxWidthFrac
	minW := r.content.MinSize().Width
	if bubbleW < minW {
		bubbleW = minW
	}
	if bubbleW > size.Width {
		bubbleW = size.Width
	}

	if r.card.isUser {
		r.content.Resize(fyne.NewSize(bubbleW, size.Height))
		r.content.Move(fyne.NewPos(size.Width-bubbleW, 0))
	} else {
		r.content.Resize(fyne.NewSize(bubbleW, size.Height))
		r.content.Move(fyne.NewPos(0, 0))
	}
}

func (r *chatCardRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *chatCardRenderer) Refresh() {
	r.bg.FillColor = theme.InputBackgroundColor()
	canvas.Refresh(r.bg)
}

func (r *chatCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *chatCardRenderer) Destroy() {}
