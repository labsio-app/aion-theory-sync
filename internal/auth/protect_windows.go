//go:build windows

package auth

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Windows DPAPI (crypt32.dll CryptProtectData/CryptUnprotectData), called
// through the standard library's syscall package so the tool stays
// dependency-free (no golang.org/x/sys module, F16). DPAPI ties the
// ciphertext to the current Windows user account: the credential cannot be
// read by copying config.json to another machine or another account.

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(data []byte) dataBlob {
	if len(data) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
}

func (b dataBlob) bytes() []byte {
	if b.pbData == nil || b.cbData == 0 {
		return nil
	}
	out := make([]byte, b.cbData)
	copy(out, unsafe.Slice(b.pbData, b.cbData))
	return out
}

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

func Protect(plain []byte) ([]byte, error) {
	in := newBlob(plain)
	var out dataBlob
	ret, _, _ := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptProtectData failed")
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}

func Unprotect(protected []byte) ([]byte, error) {
	in := newBlob(protected)
	var out dataBlob
	ret, _, _ := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed")
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}
