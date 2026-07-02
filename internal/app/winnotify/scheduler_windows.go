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

	"gix/internal/app/log"
)

// aumid deve casar com o AppUserModelID que o serviço de notificações do Wails
// registra no Windows: ele usa application.Options.Name ("gix") como AppID e cria
// o activator COM em Software\Classes\AppUserModelId\gix. Agendar toast sob outro
// AUMID (ex.: o productIdentifier) faz o Windows aceitar o AddToSchedule mas nunca
// exibir, porque esse ID não é uma fonte de toast registrada.
const aumid = "gix"

// winRTEpochOffset: ticks de 100ns entre 1601-01-01 (WinRT) e 1970-01-01 (Unix).
const winRTEpochOffset = int64(11644473600) * 1e7

type winNotifier struct{}

// New retorna o Notifier real do Windows (ScheduledToastNotification via WinRT).
func New() Notifier {
	log.Printf("winnotify.New: creating Windows notifier")
	return winNotifier{}
}

// Arm agenda um toast nativo que dispara em occ.FireAt, mesmo com o app fechado.
func (winNotifier) Arm(occ Occurrence) error {
	log.Printf("winnotify.Arm: id=%d fireAt=%s message=%q", occ.AlertID, occ.FireAt.Format(time.RFC3339), occ.Message)
	defer ensureCOM()()
	doc, err := newXmlDoc(toastXML(occ.Message))
	if err != nil {
		log.Printf("winnotify.Arm: id=%d newXmlDoc ERROR: %v", occ.AlertID, err)
		return fmt.Errorf("winnotify.Arm newXmlDoc: %w", err)
	}
	defer doc.Release()
	st, err := createScheduledToast(doc, toDateTime(occ.FireAt))
	if err != nil {
		log.Printf("winnotify.Arm: id=%d createScheduledToast ERROR: %v", occ.AlertID, err)
		return fmt.Errorf("winnotify.Arm createScheduledToast: %w", err)
	}
	defer st.Release()
	tag := strconv.FormatInt(occ.AlertID, 10)
	group := strconv.FormatInt(occ.FireAt.Unix(), 10)
	if err := setTagGroup(st, tag, group); err != nil {
		log.Printf("winnotify.Arm: id=%d setTagGroup ERROR: %v", occ.AlertID, err)
		return fmt.Errorf("winnotify.Arm setTagGroup: %w", err)
	}
	n, err := getNotifier()
	if err != nil {
		log.Printf("winnotify.Arm: id=%d getNotifier ERROR: %v", occ.AlertID, err)
		return fmt.Errorf("winnotify.Arm getNotifier: %w", err)
	}
	defer n.Release()
	err = addToSchedule(n, st)
	if err != nil {
		log.Printf("winnotify.Arm: id=%d addToSchedule ERROR: %v", occ.AlertID, err)
		return err
	}
	log.Printf("winnotify.Arm: id=%d SCHEDULED ok fireAt=%s", occ.AlertID, occ.FireAt.Format(time.RFC3339))
	return nil
}

// CancelByAlert remove todos os toasts agendados para o alertID fornecido.
func (winNotifier) CancelByAlert(alertID int64) error {
	log.Printf("winnotify.CancelByAlert: called for id=%d", alertID)
	defer ensureCOM()()
	n, err := getNotifier()
	if err != nil {
		log.Printf("winnotify.CancelByAlert: id=%d getNotifier ERROR: %v", alertID, err)
		return fmt.Errorf("winnotify.CancelByAlert getNotifier: %w", err)
	}
	defer n.Release()
	vec, err := scheduledList(n)
	if err != nil {
		log.Printf("winnotify.CancelByAlert: id=%d scheduledList ERROR: %v", alertID, err)
		return err
	}
	defer vec.Release()
	size, err := vecSize(vec)
	if err != nil {
		return err
	}
	log.Printf("winnotify.CancelByAlert: id=%d currently %d scheduled", alertID, size)
	tag := strconv.FormatInt(alertID, 10)
	for i := uint32(0); i < size; i++ {
		// Corpo em closure para liberar cada item ao fim da iteração (defer dentro
		// do for acumularia todas as refs até o retorno da função).
		if err := func() error {
			item, err := vecAt(vec, i)
			if err != nil {
				return err
			}
			defer item.Release()
			t, _, err := getTagGroup(item)
			if err != nil {
				return err
			}
			if t != tag {
				return nil
			}
			if err := removeFromSchedule(n, item); err != nil {
				log.Printf("winnotify.CancelByAlert: id=%d removeFromSchedule ERROR: %v", alertID, err)
				return fmt.Errorf("winnotify.CancelByAlert: %w", err)
			}
			return nil
		}(); err != nil {
			return err
		}
	}
	log.Printf("winnotify.CancelByAlert: id=%d done", alertID)
	return nil
}

// ListArmed devolve as chaves de todos os toasts atualmente agendados.
func (winNotifier) ListArmed() ([]Key, error) {
	defer ensureCOM()()
	n, err := getNotifier()
	if err != nil {
		log.Printf("winnotify.ListArmed: getNotifier ERROR: %v", err)
		return nil, fmt.Errorf("winnotify.ListArmed getNotifier: %w", err)
	}
	defer n.Release()
	vec, err := scheduledList(n)
	if err != nil {
		log.Printf("winnotify.ListArmed: scheduledList ERROR: %v", err)
		return nil, err
	}
	defer vec.Release()
	size, err := vecSize(vec)
	if err != nil {
		return nil, err
	}
	out := make([]Key, 0, size)
	for i := uint32(0); i < size; i++ {
		// Corpo em closure para liberar cada item ao fim da iteração.
		if err := func() error {
			item, err := vecAt(vec, i)
			if err != nil {
				return err
			}
			defer item.Release()
			tag, grp, err := getTagGroup(item)
			if err != nil {
				return err
			}
			id, _ := strconv.ParseInt(tag, 10, 64)
			unix, _ := strconv.ParseInt(grp, 10, 64)
			out = append(out, Key{AlertID: id, FireAtUnix: unix})
			return nil
		}(); err != nil {
			return nil, err
		}
	}
	log.Printf("winnotify.ListArmed: returned %d items", len(out))
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
