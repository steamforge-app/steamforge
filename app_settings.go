package main

import (
	"errors"
	"os/exec"
	"runtime"

	"steamforge/internal/logging"
	"steamforge/internal/settings"
)

func (a *App) GetSettings() settings.Settings {
	return settings.Get()
}

func (a *App) SaveSettings(newSettings settings.Settings) error {
	return settings.Save(newSettings)
}

func (a *App) GetDataDir() string {
	return settings.DataDir()
}

// OpenDataDir opens the data directory in the OS file manager.
func (a *App) OpenDataDir() error {
	dir := settings.DataDir()
	if dir == "" {
		return errors.New("data directory not set")
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", dir).Start()
	case "darwin":
		return exec.Command("open", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}

// GetLogContent returns the contents of the current session's log file.
func (a *App) GetLogContent() (string, error) {
	return logging.ReadLog()
}
