//go:build windows

package steam

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// GetInstallPath returns the Steam installation path on Windows.
func GetInstallPath() (string, error) {
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		path, err := getInstallPathFromKey(root)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Steam install path not found in registry")
}

func getInstallPathFromKey(root registry.Key) (string, error) {
	key, err := registry.OpenKey(root, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	path, _, err := key.GetStringValue("InstallPath")
	if err != nil {
		return "", err
	}
	return path, nil
}
