//go:build windows

package autostart

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Raw advapi32.dll registry calls, called through the standard library's
// syscall package — the same choice internal/auth/protect_windows.go
// makes, and for the same reason: this stays dependency-free rather than
// pulling in golang.org/x/sys/windows/registry for one key and one value.
var (
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

const (
	hkeyCurrentUser = 0x80000001

	keyQueryValue = 0x0001
	keySetValue   = 0x0002

	regSZ = 1

	errorSuccess      = 0
	errorFileNotFound = 2
	errorMoreData     = 234
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const valueName = "AionTheorySync"

// Windows is the real Autostart, backed by the Run registry key.
type Windows struct{}

func (Windows) Enable(exePath string, minimized bool) error {
	key, err := openRunKey(keySetValue)
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(uintptr(key))

	data, err := syscall.UTF16FromString(commandLine(exePath, minimized))
	if err != nil {
		return err
	}
	name, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return err
	}

	ret, _, _ := procRegSetValueExW.Call(
		uintptr(key),
		uintptr(unsafe.Pointer(name)),
		0,
		regSZ,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)*2), // UTF-16 code units, in bytes, including the terminating NUL
	)
	if ret != errorSuccess {
		return fmt.Errorf("RegSetValueExW failed: %#x", ret)
	}
	return nil
}

func (Windows) Disable() error {
	key, err := openRunKey(keySetValue)
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(uintptr(key))

	name, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return err
	}
	ret, _, _ := procRegDeleteValueW.Call(uintptr(key), uintptr(unsafe.Pointer(name)))
	if ret != errorSuccess && ret != errorFileNotFound {
		return fmt.Errorf("RegDeleteValueW failed: %#x", ret)
	}
	return nil
}

func (Windows) IsEnabled() (bool, error) {
	key, err := openRunKey(keyQueryValue)
	if err != nil {
		return false, err
	}
	defer procRegCloseKey.Call(uintptr(key))

	name, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return false, err
	}
	var size uint32
	ret, _, _ := procRegQueryValueExW.Call(
		uintptr(key), uintptr(unsafe.Pointer(name)), 0, 0, 0, uintptr(unsafe.Pointer(&size)),
	)
	if ret == errorFileNotFound {
		return false, nil
	}
	if ret != errorSuccess && ret != errorMoreData {
		return false, fmt.Errorf("RegQueryValueExW (size probe) failed: %#x", ret)
	}
	return true, nil
}

// openRunKey opens HKCU\...\Run with the given access mask. The Run key
// itself always exists on Windows, so RegOpenKeyExW (not
// RegCreateKeyExW) is enough.
func openRunKey(access uint32) (syscall.Handle, error) {
	path, err := syscall.UTF16PtrFromString(runKeyPath)
	if err != nil {
		return 0, err
	}
	var key syscall.Handle
	ret, _, _ := procRegOpenKeyExW.Call(
		uintptr(hkeyCurrentUser),
		uintptr(unsafe.Pointer(path)),
		0,
		uintptr(access),
		uintptr(unsafe.Pointer(&key)),
	)
	if ret != errorSuccess {
		return 0, fmt.Errorf("RegOpenKeyExW(%s) failed: %#x", runKeyPath, ret)
	}
	return key, nil
}
