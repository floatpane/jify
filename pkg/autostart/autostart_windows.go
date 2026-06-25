//go:build windows

package autostart

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	advapi32          = syscall.NewLazyDLL("advapi32.dll")
	pRegOpenKeyEx     = advapi32.NewProc("RegOpenKeyExW")
	pRegSetValueEx    = advapi32.NewProc("RegSetValueExW")
	pRegQueryValueEx  = advapi32.NewProc("RegQueryValueExW")
	pRegDeleteValueEx = advapi32.NewProc("RegDeleteValueW")
	pRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

const (
	hkeyCurrentUser = 0x80000001
	keyQueryValue   = 0x0001
	keySetValue     = 0x0002
	regSz           = 1
	runSubKey       = `Software\Microsoft\Windows\CurrentVersion\Run`
)

func openRunKey(access uint32) (syscall.Handle, error) {
	sub, err := syscall.UTF16PtrFromString(runSubKey)
	if err != nil {
		return 0, err
	}
	var h syscall.Handle
	ret, _, _ := pRegOpenKeyEx.Call(
		uintptr(hkeyCurrentUser),
		uintptr(unsafe.Pointer(sub)),
		0,
		uintptr(access),
		uintptr(unsafe.Pointer(&h)),
	)
	if ret != 0 {
		return 0, fmt.Errorf("RegOpenKeyEx failed: %d", ret)
	}
	return h, nil
}

// Enable adds an HKCU Run registry value pointing at the jify binary.
func Enable() error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	h, err := openRunKey(keySetValue)
	if err != nil {
		return err
	}
	defer pRegCloseKey.Call(uintptr(h))

	name, err := syscall.UTF16PtrFromString(appName)
	if err != nil {
		return err
	}
	// Quote the path so spaces in the install location are handled.
	data, err := syscall.UTF16FromString(`"` + exe + `"`)
	if err != nil {
		return err
	}
	ret, _, _ := pRegSetValueEx.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(name)),
		0,
		regSz,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)*2), // bytes, including the trailing NUL
	)
	if ret != 0 {
		return fmt.Errorf("RegSetValueEx failed: %d", ret)
	}
	return nil
}

// Disable removes the HKCU Run registry value.
func Disable() error {
	h, err := openRunKey(keySetValue)
	if err != nil {
		return err
	}
	defer pRegCloseKey.Call(uintptr(h))

	name, err := syscall.UTF16PtrFromString(appName)
	if err != nil {
		return err
	}
	ret, _, _ := pRegDeleteValueEx.Call(uintptr(h), uintptr(unsafe.Pointer(name)))
	// 2 == ERROR_FILE_NOT_FOUND (already absent) is fine.
	if ret != 0 && ret != 2 {
		return fmt.Errorf("RegDeleteValue failed: %d", ret)
	}
	return nil
}

// IsEnabled reports whether the HKCU Run registry value exists.
func IsEnabled() (bool, error) {
	h, err := openRunKey(keyQueryValue)
	if err != nil {
		return false, err
	}
	defer pRegCloseKey.Call(uintptr(h))

	name, err := syscall.UTF16PtrFromString(appName)
	if err != nil {
		return false, err
	}
	ret, _, _ := pRegQueryValueEx.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(name)),
		0, 0, 0, 0,
	)
	if ret == 0 {
		return true, nil
	}
	if ret == 2 { // ERROR_FILE_NOT_FOUND
		return false, nil
	}
	return false, fmt.Errorf("RegQueryValueEx failed: %d", ret)
}
