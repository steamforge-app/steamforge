//go:build windows

package steam

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

type windowsLibrary struct {
	dll *syscall.DLL
}

// OpenLibrary loads a DLL on Windows using LoadLibraryEx.
func OpenLibrary(path string) (Library, error) {
	// Add the directory to the DLL search path
	dir := filepath.Dir(path)
	dirPtr, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return nil, fmt.Errorf("utf16 dir: %w", err)
	}
	kernel32 := syscall.MustLoadDLL("kernel32.dll")
	setDllDirectory := kernel32.MustFindProc("SetDllDirectoryW")
	setDllDirectory.Call(uintptr(unsafe.Pointer(dirPtr)))

	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, fmt.Errorf("LoadDLL %s: %w", path, err)
	}
	return &windowsLibrary{dll: dll}, nil
}

func (l *windowsLibrary) FindProc(name string) (uintptr, error) {
	proc, err := l.dll.FindProc(name)
	if err != nil {
		return 0, fmt.Errorf("FindProc %s: %w", name, err)
	}
	return proc.Addr(), nil
}

func (l *windowsLibrary) Close() error {
	return l.dll.Release()
}

// SteamClientLibraryPath returns the path to steamclient64.dll on Windows.
func SteamClientLibraryPath() (string, error) {
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		dllPath, err := steamClientDLLFromKey(root)
		if err == nil {
			return dllPath, nil
		}
	}

	return "", fmt.Errorf("Steam install path not found in registry")
}

func steamClientDLLFromKey(root registry.Key) (string, error) {
	key, err := registry.OpenKey(root, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	installPath, _, err := key.GetStringValue("InstallPath")
	if err != nil {
		return "", err
	}

	return filepath.Join(installPath, "steamclient64.dll"), nil
}

// CallProc calls a function pointer with the given arguments using syscall.SyscallN.
func CallProc(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(fn, args...)
	return r
}
