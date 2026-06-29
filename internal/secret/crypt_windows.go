//go:build windows

package secret

import (
	"syscall"
	"unsafe"
)

// DPAPI (Data Protection API): cifra/decifra ligado à conta do usuário logado.
// Sem chave para gerenciar — o SO deriva do perfil do usuário; outro usuário (ou
// outra máquina) não consegue decifrar o blob.
var (
	crypt32            = syscall.NewLazyDLL("crypt32.dll")
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtect   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree      = kernel32.NewProc("LocalFree")
)

// CRYPTPROTECT_UI_FORBIDDEN: nunca exibe prompt de UI (rodamos sem desktop ativo).
const cryptProtectUIForbidden = 0x1

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) dataBlob {
	if len(d) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(d)), pbData: &d[0]}
}

func (b dataBlob) bytes() []byte {
	out := make([]byte, b.cbData)
	copy(out, unsafe.Slice(b.pbData, b.cbData))
	return out
}

func protect(data []byte) ([]byte, error) {
	in := newBlob(data)
	var out dataBlob
	ret, _, err := procCryptProtect.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0,
		cryptProtectUIForbidden, uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}

func unprotect(data []byte) ([]byte, error) {
	in := newBlob(data)
	var out dataBlob
	ret, _, err := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0,
		cryptProtectUIForbidden, uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}
