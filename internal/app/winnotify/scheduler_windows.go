//go:build windows

// Package winnotify — implementação Windows do Notifier usando WinRT via go-ole.
// Este arquivo expõe New() e os três métodos; os bindings COM brutos vivem em
// winrt_windows.go (separados por responsabilidade).
package winnotify

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/saltosystems/winrt-go/windows/foundation"
)

// aumid deve casar com o AppUserModelID que o instalador registra.
const aumid = "com.gix.app"

// winRTEpochOffset: ticks de 100ns entre 1601-01-01 (WinRT) e 1970-01-01 (Unix).
const winRTEpochOffset = int64(11644473600) * 1e7

type winNotifier struct{}

// New retorna o Notifier real do Windows (ScheduledToastNotification via WinRT).
func New() Notifier { return winNotifier{} }

// Arm agenda um toast nativo que dispara em occ.FireAt, mesmo com o app fechado.
func (winNotifier) Arm(occ Occurrence) error {
	doc, err := newXmlDoc(toastXML(occ.Message))
	if err != nil {
		return fmt.Errorf("winnotify.Arm newXmlDoc: %w", err)
	}
	st, err := createScheduledToast(doc, toDateTime(occ.FireAt))
	if err != nil {
		return fmt.Errorf("winnotify.Arm createScheduledToast: %w", err)
	}
	tag := strconv.FormatInt(occ.AlertID, 10)
	group := strconv.FormatInt(occ.FireAt.Unix(), 10)
	if err := setTagGroup(st, tag, group); err != nil {
		return fmt.Errorf("winnotify.Arm setTagGroup: %w", err)
	}
	n, err := getNotifier()
	if err != nil {
		return fmt.Errorf("winnotify.Arm getNotifier: %w", err)
	}
	return addToSchedule(n, st)
}

// CancelByAlert remove todos os toasts agendados para o alertID fornecido.
func (winNotifier) CancelByAlert(alertID int64) error {
	n, err := getNotifier()
	if err != nil {
		return fmt.Errorf("winnotify.CancelByAlert getNotifier: %w", err)
	}
	vec, err := scheduledList(n)
	if err != nil {
		return err
	}
	size, err := vecSize(vec)
	if err != nil {
		return err
	}
	tag := strconv.FormatInt(alertID, 10)
	for i := uint32(0); i < size; i++ {
		item, err := vecAt(vec, i)
		if err != nil {
			return err
		}
		t, _, err := getTagGroup(item)
		if err != nil {
			return err
		}
		if t != tag {
			continue
		}
		if err := removeFromSchedule(n, item); err != nil {
			return fmt.Errorf("winnotify.CancelByAlert: %w", err)
		}
	}
	return nil
}

// ListArmed devolve as chaves de todos os toasts atualmente agendados.
func (winNotifier) ListArmed() ([]Key, error) {
	n, err := getNotifier()
	if err != nil {
		return nil, fmt.Errorf("winnotify.ListArmed getNotifier: %w", err)
	}
	vec, err := scheduledList(n)
	if err != nil {
		return nil, err
	}
	size, err := vecSize(vec)
	if err != nil {
		return nil, err
	}
	out := make([]Key, 0, size)
	for i := uint32(0); i < size; i++ {
		item, err := vecAt(vec, i)
		if err != nil {
			return nil, err
		}
		tag, grp, err := getTagGroup(item)
		if err != nil {
			return nil, err
		}
		id, _ := strconv.ParseInt(tag, 10, 64)
		unix, _ := strconv.ParseInt(grp, 10, 64)
		out = append(out, Key{AlertID: id, FireAtUnix: unix})
	}
	return out, nil
}

// ─── Utilitários ─────────────────────────────────────────────────────────────

// toDateTime converte time.Time → foundation.DateTime (100ns ticks desde 1601-01-01).
func toDateTime(t time.Time) foundation.DateTime {
	return foundation.DateTime{
		UniversalTime: t.Unix()*1e7 + int64(t.Nanosecond())/100 + winRTEpochOffset,
	}
}

// toastXML produz o XML de toast com título "gix" e corpo message (XML-escaped).
func toastXML(message string) string {
	return `<toast><visual><binding template="ToastGeneric">` +
		`<text>gix</text><text>` + xmlEsc(message) + `</text>` +
		`</binding></visual></toast>`
}

func xmlEsc(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
