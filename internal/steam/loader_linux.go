//go:build linux

package steam

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
)

type linuxLibrary struct {
	handle uintptr
}

// OpenLibrary loads a shared library on Linux using purego (dlopen).
func OpenLibrary(path string) (Library, error) {
	handle, err := purego.Dlopen(path, purego.RTLD_LAZY)
	if err != nil {
		return nil, fmt.Errorf("dlopen %s: %w", path, err)
	}
	return &linuxLibrary{handle: handle}, nil
}

func (l *linuxLibrary) FindProc(name string) (uintptr, error) {
	sym, err := purego.Dlsym(l.handle, name)
	if err != nil {
		return 0, fmt.Errorf("dlsym %s: %w", name, err)
	}
	return sym, nil
}

func (l *linuxLibrary) Close() error {
	// purego doesn't expose dlclose, so we just nil the handle
	l.handle = 0
	return nil
}

// SteamClientLibraryPath returns the path to steamclient.so on Linux.
func SteamClientLibraryPath() (string, error) {
	// Check common Steam install locations
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	candidates := []string{
		filepath.Join(home, ".steam", "sdk64", "steamclient.so"),
		filepath.Join(home, ".local", "share", "Steam", "linux64", "steamclient.so"),
		filepath.Join(home, ".steam", "steam", "linux64", "steamclient.so"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("steamclient.so not found in any standard location")
}

// CallProc calls a function pointer with the given arguments.
// On Linux we use purego.SyscallN which handles register allocation properly.
func CallProc(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}
