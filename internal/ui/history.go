package ui

import (
	"gix/internal/db"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func showHistoryWindow(a fyne.App) {
	hw := a.NewWindow(getTr("history"))

	showEmpty := func() {
		hw.SetContent(container.NewPadded(widget.NewLabel(getTr("history_empty"))))
		hw.Resize(fyne.NewSize(420, 200))
		hw.CenterOnScreen()
		hw.Show()
	}

	if database == nil {
		showEmpty()
		return
	}
	convs, err := database.ListConversations()
	if err != nil || len(convs) == 0 {
		showEmpty()
		return
	}

	detail := container.NewVBox()
	detailScroll := container.NewVScroll(detail)

	showConversation := func(c db.Conversation) {
		detail.RemoveAll()
		msgs, err := database.GetMessages(c.ID)
		if err != nil {
			detail.Add(widget.NewLabel(getTr("error_prefix") + err.Error()))
			detail.Refresh()
			return
		}
		for _, m := range msgs {
			isUser := m.Role == "user"
			card := newChatCard(m.Content, isUser)
			detail.Add(card)
		}
		detail.Refresh()
		detailScroll.ScrollToTop()
	}

	var list *widget.List
	list = widget.NewList(
		func() int { return len(convs) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			delBtn := newIconButton(theme.DeleteIcon(), nil)
			delBtn.Importance = widget.DangerImportance
			return container.NewBorder(nil, nil, nil, delBtn, label)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(convs) {
				return
			}
			c := convs[id]
			row := item.(*fyne.Container)
			label := row.Objects[0].(*widget.Label)
			delBtn := row.Objects[1].(*cursorButton)
			label.SetText(c.Title)
			delBtn.OnTapped = func() {
				_ = database.DeleteConversation(c.ID)
				convs, _ = database.ListConversations()
				detail.RemoveAll()
				detail.Refresh()
				list.UnselectAll()
				list.Refresh()
			}
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id < len(convs) {
			showConversation(convs[id])
		}
	}

	split := container.NewHSplit(list, detailScroll)
	split.SetOffset(0.4)
	hw.SetContent(split)
	hw.Resize(fyne.NewSize(640, 480))
	hw.CenterOnScreen()
	hw.Show()
}
