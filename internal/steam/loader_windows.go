//go:build windows

package steam

import (
	"fmt"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type windowsLibrary struct {
	handle windows.Handle
}

// OpenLibrary loads a DLL on Windows using LoadLibraryEx with
// LOAD_WITH_ALTERED_SEARCH_PATH so the DLL resolves its own dependencies
// from its directory — without mutating the global DLL search path.
func OpenLibrary(path string) (Library, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path %s: %w", path, err)
	}

	handle, err := windows.LoadLibraryEx(absPath, 0, windows.LOAD_WITH_ALTERED_SEARCH_PATH)
	if err != nil {
		return nil, fmt.Errorf("LoadLibraryEx %s: %w", absPath, err)
	}
	return &windowsLibrary{handle: handle}, nil
}

func (l *windowsLibrary) FindProc(name string) (uintptr, error) {
	proc, err := syscall.GetProcAddress(syscall.Handle(l.handle), name)
	if err != nil {
		return 0, fmt.Errorf("GetProcAddress %s: %w", name, err)
	}
	return proc, nil
}

func (l *windowsLibrary) Close() error {
	return windows.FreeLibrary(l.handle)
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
