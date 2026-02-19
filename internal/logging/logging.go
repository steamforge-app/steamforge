package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	logFileName    = "steamforge.log"
	dirPermissions = 0755
)

// Setup initializes structured logging to a file.
// Returns the log file path, a cleanup function to close the file, and any error.
// The global slog.Default() logger is set to write to the log file.
func Setup() (string, func(), error) {
	logDir, err := logDirectory()
	if err != nil {
		return "", nil, fmt.Errorf("determine log directory: %w", err)
	}

	if err := os.MkdirAll(logDir, dirPermissions); err != nil {
		return "", nil, fmt.Errorf("create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, logFileName)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", nil, fmt.Errorf("open log file: %w", err)
	}

	handler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))

	cleanup := func() {
		f.Close()
	}

	return logPath, cleanup, nil
}

// LogPath returns the current log file path.
func LogPath() string {
	dir, err := logDirectory()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, logFileName)
}

// ReadLog returns the contents of the current log file.
func ReadLog() (string, error) {
	path := LogPath()
	if path == "" {
		return "", fmt.Errorf("log path not available")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read log file: %w", err)
	}
	return string(data), nil
}

func logDirectory() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config dir: %w", err)
	}
	return filepath.Join(dir, "steamforge"), nil
}
