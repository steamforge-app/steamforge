package steam

import (
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// InstallWatcher monitors steamapps directories for appmanifest changes
// and invokes a callback with the current set of installed app IDs.
type InstallWatcher struct {
	watcher  *fsnotify.Watcher
	callback func(installed map[uint32]bool)
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewInstallWatcher creates a watcher that monitors all Steam library steamapps
// directories for appmanifest file changes. When changes are detected (debounced
// to ~1 second), it re-scans installed games and invokes the callback.
func NewInstallWatcher(callback func(installed map[uint32]bool)) (*InstallWatcher, error) {
	installPath, err := GetInstallPath()
	if err != nil {
		return nil, err
	}

	libraryPaths := collectLibraryPaths(installPath)

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	watched := 0
	for _, libPath := range libraryPaths {
		dir := filepath.Join(libPath, "steamapps")
		if err := fsWatcher.Add(dir); err != nil {
			slog.Warn("install watcher: cannot watch directory", "path", dir, "error", err)
			continue
		}
		watched++
		slog.Info("install watcher: watching directory", "path", dir)
	}

	if watched == 0 {
		fsWatcher.Close()
		return nil, errors.New("no steamapps directories found to watch")
	}

	w := &InstallWatcher{
		watcher:  fsWatcher,
		callback: callback,
		stopCh:   make(chan struct{}),
	}

	w.wg.Add(1)
	go w.loop()

	slog.Info("install watcher started", "directories", watched)
	return w, nil
}

func (w *InstallWatcher) loop() {
	defer w.wg.Done()

	var debounceTimer *time.Timer
	var debounceC <-chan time.Time

	for {
		select {
		case <-w.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if !isAppManifestEvent(event) {
				continue
			}
			slog.Debug("install watcher: appmanifest change", "op", event.Op, "path", event.Name)

			// Reset debounce timer — batch rapid changes into a single re-scan.
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(1 * time.Second)
			debounceC = debounceTimer.C

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("install watcher error", "error", err)

		case <-debounceC:
			debounceTimer = nil
			debounceC = nil
			w.rescan()
		}
	}
}

func (w *InstallWatcher) rescan() {
	games, err := ScanInstalledGames()
	if err != nil {
		slog.Warn("install watcher: rescan failed", "error", err)
		return
	}

	installed := make(map[uint32]bool, len(games))
	for _, g := range games {
		installed[g.AppID] = true
	}

	slog.Info("install watcher: rescan complete", "installed", len(installed))
	w.callback(installed)
}

// isAppManifestEvent returns true if the event is for an appmanifest_*.acf file
// and is a create, remove, or rename operation.
func isAppManifestEvent(event fsnotify.Event) bool {
	base := filepath.Base(event.Name)
	if !strings.HasPrefix(base, "appmanifest_") || !strings.HasSuffix(base, ".acf") {
		return false
	}
	return event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0
}

// Close stops the watcher and releases resources.
func (w *InstallWatcher) Close() {
	close(w.stopCh)
	w.watcher.Close()
	w.wg.Wait()
	slog.Info("install watcher stopped")
}
