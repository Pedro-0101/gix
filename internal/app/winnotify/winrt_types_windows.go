//go:build windows

// winrt_types_windows.go — structs COM/WinRT e acessores vtbl usados por winrt_windows.go.
package winnotify

import (
	"unsafe"

	"github.com/go-ole/go-ole"
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
