//go:build linux

package steam

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetInstallPath returns the Steam installation path on Linux.
func GetInstallPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	candidates := []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam"),
	}

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("Steam installation not found")
}
