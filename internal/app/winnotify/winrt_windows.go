//go:build windows

// Package winnotify — bindings COM/WinRT brutos via go-ole.
// GUIDs verificados via PowerShell reflection contra Windows.UI.winmd (2026-06-30).
package winnotify

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

// GUIDs verificados via PowerShell reflection (vários valores do brief estavam errados).
const (
	guidXmlDocumentIO     = "6cd0e74e-ee65-4489-9ebf-ca43e87ba637" // IXmlDocumentIO
	guidToastMgrStatics   = "50ac103f-d235-4598-bbef-98fe4d1a3ad4" // IToastNotificationManagerStatics
	guidToastNotifier     = "75927b93-03f3-41ec-91d3-6e5bac1b38e7" // IToastNotifier
	guidSchedToastFactory = "e7bed191-0bb9-4189-8394-31761b476fd7" // IScheduledToastNotificationFactory
	guidSchedToast2       = "a66ea09c-31b4-43b0-b5dd-7a40e85363b1" // IScheduledToastNotification2
)

type iXmlDocumentIO struct{ ole.IInspectable }
type iXmlDocumentIOVtbl struct {
	ole.IInspectableVtbl
	LoadXml             uintptr
	LoadXmlWithSettings uintptr
	SaveToFileAsync     uintptr
}

func (v *iXmlDocumentIO) vtbl() *iXmlDocumentIOVtbl {
	return (*iXmlDocumentIOVtbl)(unsafe.Pointer(v.RawVTable))
}

type iToastMgrStatics struct{ ole.IInspectable }
type iToastMgrStaticsVtbl struct {
	ole.IInspectableVtbl
	CreateToastNotifier       uintptr
	CreateToastNotifierWithId uintptr
	GetTemplateContent        uintptr
}

func (v *iToastMgrStatics) vtbl() *iToastMgrStaticsVtbl {
	return (*iToastMgrStaticsVtbl)(unsafe.Pointer(v.RawVTable))
}

type iToastNotifier struct{ ole.IInspectable }
type iToastNotifierVtbl struct {
	ole.IInspectableVtbl
	Show                           uintptr
	Hide                           uintptr
	GetSetting                     uintptr
	AddToSchedule                  uintptr
	RemoveFromSchedule             uintptr
	GetScheduledToastNotifications uintptr
}

func (v *iToastNotifier) vtbl() *iToastNotifierVtbl {
	return (*iToastNotifierVtbl)(unsafe.Pointer(v.RawVTable))
}

type iSchedToastFactory struct{ ole.IInspectable }
type iSchedToastFactoryVtbl struct {
	ole.IInspectableVtbl
	CreateScheduledToastNotification          uintptr
	CreateScheduledToastNotificationRecurring uintptr
}

func (v *iSchedToastFactory) vtbl() *iSchedToastFactoryVtbl {
	return (*iSchedToastFactoryVtbl)(unsafe.Pointer(v.RawVTable))
}

// iSchedToast2: IScheduledToastNotification2; vtable: put_Tag, get_Tag, put_Group, get_Group, ...
type iSchedToast2 struct{ ole.IInspectable }
type iSchedToast2Vtbl struct {
	ole.IInspectableVtbl
	PutTag           uintptr
	GetTag           uintptr
	PutGroup         uintptr
	GetGroup         uintptr
	PutSuppressPopup uintptr
	GetSuppressPopup uintptr
}

func (v *iSchedToast2) vtbl() *iSchedToast2Vtbl {
	return (*iSchedToast2Vtbl)(unsafe.Pointer(v.RawVTable))
}

type vecView struct{ ole.IInspectable } // IVectorView<ScheduledToastNotification>
type vecViewVtbl struct {
	ole.IInspectableVtbl
	GetAt   uintptr
	GetSize uintptr
	IndexOf uintptr
	GetMany uintptr
}

func (v *vecView) vtbl() *vecViewVtbl {
	return (*vecViewVtbl)(unsafe.Pointer(v.RawVTable))
}

type schedToastObj struct{ ole.IUnknown } // IScheduledToastNotification*
type xmlDocObj struct{ ole.IUnknown }     // IXmlDocument*

func init() {
	// Inicializa WinRT (MTA); erro ignorado — Wails pode já ter inicializado.
	_ = ole.RoInitialize(1)
}

// getNotifier obtém o IToastNotifier para o aumid registrado.
func getNotifier() (*iToastNotifier, error) {
	factory, err := ole.RoGetActivationFactory(
		"Windows.UI.Notifications.ToastNotificationManager",
		ole.NewGUID(guidToastMgrStatics),
	)
	if err != nil {
		return nil, fmt.Errorf("RoGetActivationFactory ToastNotificationManager: %w", err)
	}
	statics := (*iToastMgrStatics)(unsafe.Pointer(factory))

	aumidH, err := ole.NewHString(aumid)
	if err != nil {
		return nil, err
	}
	defer ole.DeleteHString(aumidH) //nolint:errcheck

	var rawNotifier unsafe.Pointer
	hr, _, _ := syscall.SyscallN(
		statics.vtbl().CreateToastNotifierWithId,
		uintptr(unsafe.Pointer(statics)),
		uintptr(aumidH),
		uintptr(unsafe.Pointer(&rawNotifier)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("CreateToastNotifierWithId: %w", ole.NewError(hr))
	}
	return (*iToastNotifier)(rawNotifier), nil
}

// newXmlDoc cria um XmlDocument WinRT e carrega o XML.
func newXmlDoc(xml string) (*xmlDocObj, error) {
	inspectable, err := ole.RoActivateInstance("Windows.Data.Xml.Dom.XmlDocument")
	if err != nil {
		return nil, fmt.Errorf("RoActivateInstance XmlDocument: %w", err)
	}
	doc := (*xmlDocObj)(unsafe.Pointer(inspectable))

	ioDisp, err := doc.QueryInterface(ole.NewGUID(guidXmlDocumentIO))
	if err != nil {
		return nil, fmt.Errorf("QI IXmlDocumentIO: %w", err)
	}
	defer ioDisp.Release()
	io := (*iXmlDocumentIO)(unsafe.Pointer(ioDisp))

	hstr, err := ole.NewHString(xml)
	if err != nil {
		return nil, err
	}
	defer ole.DeleteHString(hstr) //nolint:errcheck

	hr, _, _ := syscall.SyscallN(io.vtbl().LoadXml, uintptr(unsafe.Pointer(io)), uintptr(hstr))
	if hr != 0 {
		return nil, fmt.Errorf("LoadXml: %w", ole.NewError(hr))
	}
	return doc, nil
}

// createScheduledToast cria um ScheduledToastNotification via fábrica WinRT.
func createScheduledToast(doc *xmlDocObj, dt foundation.DateTime) (*schedToastObj, error) {
	factory, err := ole.RoGetActivationFactory(
		"Windows.UI.Notifications.ScheduledToastNotification",
		ole.NewGUID(guidSchedToastFactory),
	)
	if err != nil {
		return nil, fmt.Errorf("RoGetActivationFactory ScheduledToastNotification: %w", err)
	}
	f := (*iSchedToastFactory)(unsafe.Pointer(factory))

	var rawToast unsafe.Pointer
	hr, _, _ := syscall.SyscallN(
		f.vtbl().CreateScheduledToastNotification,
		uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(doc)),
		uintptr(dt.UniversalTime),
		uintptr(unsafe.Pointer(&rawToast)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("CreateScheduledToastNotification: %w", ole.NewError(hr))
	}
	return (*schedToastObj)(rawToast), nil
}

// setTagGroup define tag (alertID) e group (fireAtUnix) via IScheduledToastNotification2.
func setTagGroup(st *schedToastObj, tag, group string) error {
	disp, err := st.QueryInterface(ole.NewGUID(guidSchedToast2))
	if err != nil {
		return fmt.Errorf("QI IScheduledToastNotification2: %w", err)
	}
	defer disp.Release()
	s2 := (*iSchedToast2)(unsafe.Pointer(disp))

	tagH, err := ole.NewHString(tag)
	if err != nil {
		return err
	}
	defer ole.DeleteHString(tagH) //nolint:errcheck
	if hr, _, _ := syscall.SyscallN(s2.vtbl().PutTag, uintptr(unsafe.Pointer(s2)), uintptr(tagH)); hr != 0 {
		return fmt.Errorf("put_Tag: %w", ole.NewError(hr))
	}

	grpH, err := ole.NewHString(group)
	if err != nil {
		return err
	}
	defer ole.DeleteHString(grpH) //nolint:errcheck
	if hr, _, _ := syscall.SyscallN(s2.vtbl().PutGroup, uintptr(unsafe.Pointer(s2)), uintptr(grpH)); hr != 0 {
		return fmt.Errorf("put_Group: %w", ole.NewError(hr))
	}
	return nil
}

// getTagGroup lê tag e group de um ScheduledToastNotification.
func getTagGroup(st *schedToastObj) (tag, group string, err error) {
	disp, err := st.QueryInterface(ole.NewGUID(guidSchedToast2))
	if err != nil {
		return "", "", fmt.Errorf("QI IScheduledToastNotification2: %w", err)
	}
	defer disp.Release()
	s2 := (*iSchedToast2)(unsafe.Pointer(disp))

	var tagH ole.HString
	if hr, _, _ := syscall.SyscallN(s2.vtbl().GetTag, uintptr(unsafe.Pointer(s2)), uintptr(unsafe.Pointer(&tagH))); hr != 0 {
		return "", "", fmt.Errorf("get_Tag: %w", ole.NewError(hr))
	}
	tag = tagH.String()
	_ = ole.DeleteHString(tagH)

	var grpH ole.HString
	if hr, _, _ := syscall.SyscallN(s2.vtbl().GetGroup, uintptr(unsafe.Pointer(s2)), uintptr(unsafe.Pointer(&grpH))); hr != 0 {
		return "", "", fmt.Errorf("get_Group: %w", ole.NewError(hr))
	}
	group = grpH.String()
	_ = ole.DeleteHString(grpH)
	return tag, group, nil
}

// scheduledList retorna o IVectorView<ScheduledToastNotification> do notifier.
func scheduledList(n *iToastNotifier) (*vecView, error) {
	var rawVec unsafe.Pointer
	hr, _, _ := syscall.SyscallN(
		n.vtbl().GetScheduledToastNotifications,
		uintptr(unsafe.Pointer(n)),
		uintptr(unsafe.Pointer(&rawVec)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("GetScheduledToastNotifications: %w", ole.NewError(hr))
	}
	return (*vecView)(rawVec), nil
}

// vecSize retorna o número de elementos do vector view.
func vecSize(v *vecView) (uint32, error) {
	var size uint32
	hr, _, _ := syscall.SyscallN(v.vtbl().GetSize, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(&size)))
	if hr != 0 {
		return 0, fmt.Errorf("IVectorView.GetSize: %w", ole.NewError(hr))
	}
	return size, nil
}

// vecAt retorna o item na posição i.
func vecAt(v *vecView, i uint32) (*schedToastObj, error) {
	var rawItem unsafe.Pointer
	hr, _, _ := syscall.SyscallN(v.vtbl().GetAt, uintptr(unsafe.Pointer(v)), uintptr(i), uintptr(unsafe.Pointer(&rawItem)))
	if hr != 0 {
		return nil, fmt.Errorf("IVectorView.GetAt(%d): %w", i, ole.NewError(hr))
	}
	return (*schedToastObj)(rawItem), nil
}

// addToSchedule chama IToastNotifier::AddToSchedule.
func addToSchedule(n *iToastNotifier, st *schedToastObj) error {
	hr, _, _ := syscall.SyscallN(n.vtbl().AddToSchedule, uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(st)))
	if hr != 0 {
		return fmt.Errorf("AddToSchedule: %w", ole.NewError(hr))
	}
	return nil
}

// removeFromSchedule chama IToastNotifier::RemoveFromSchedule.
func removeFromSchedule(n *iToastNotifier, st *schedToastObj) error {
	hr, _, _ := syscall.SyscallN(n.vtbl().RemoveFromSchedule, uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(st)))
	if hr != 0 {
		return fmt.Errorf("RemoveFromSchedule: %w", ole.NewError(hr))
	}
	return nil
}
