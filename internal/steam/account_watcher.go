package steam

import (
	"bufio"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// steamIDBase is the minimum valid Steam ID (64-bit). IDs above this threshold
// are full Steam community IDs (e.g. 76561198000000000).
const steamIDBase uint64 = 76561197960265728

// AccountWatcher monitors loginusers.vdf for Steam account changes.
// When the active user (MostRecent "1") changes, it invokes the callback
// with the new Steam ID.
type AccountWatcher struct {
	watcher   *fsnotify.Watcher
	currentID uint64
	callback  func(newSteamID uint64)
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewAccountWatcher creates a watcher that monitors loginusers.vdf for
// account switches. The currentID is the Steam ID of the currently connected user.
func NewAccountWatcher(currentID uint64, callback func(newSteamID uint64)) (*AccountWatcher, error) {
	installPath, err := GetInstallPath()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(installPath, "config")
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := fsWatcher.Add(configDir); err != nil {
		fsWatcher.Close()
		return nil, err
	}

	w := &AccountWatcher{
		watcher:   fsWatcher,
		currentID: currentID,
		callback:  callback,
		stopCh:    make(chan struct{}),
	}

	w.wg.Add(1)
	go w.loop(filepath.Join(configDir, "loginusers.vdf"))

	slog.Info("account watcher started", "steamID", currentID)
	return w, nil
}

func (w *AccountWatcher) loop(vdfPath string) {
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
			if filepath.Base(event.Name) != "loginusers.vdf" {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			slog.Debug("account watcher: loginusers.vdf changed", "op", event.Op)

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(2 * time.Second)
			debounceC = debounceTimer.C

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("account watcher error", "error", err)

		case <-debounceC:
			debounceTimer = nil
			debounceC = nil
			w.checkForChange(vdfPath)
		}
	}
}

func (w *AccountWatcher) checkForChange(vdfPath string) {
	activeID, err := ParseActiveUserID(vdfPath)
	if err != nil {
		slog.Debug("account watcher: failed to parse loginusers.vdf", "error", err)
		return
	}

	if activeID == 0 || activeID == w.currentID {
		return
	}

	slog.Info("account watcher: account changed", "old", w.currentID, "new", activeID)
	w.currentID = activeID
	w.callback(activeID)
}

// ParseActiveUserID reads loginusers.vdf and returns the Steam ID marked as MostRecent.
func ParseActiveUserID(vdfPath string) (uint64, error) {
	f, err := os.Open(vdfPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var currentKey uint64
	var mostRecentID uint64

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Match a quoted string that looks like a Steam ID (17-digit number)
		if strings.HasPrefix(line, "\"") && !strings.Contains(line, "\t") {
			trimmed := strings.Trim(line, "\"")
			if id, err := strconv.ParseUint(trimmed, 10, 64); err == nil && id > steamIDBase {
				currentKey = id
			}
			continue
		}

		m := kvPairRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if strings.EqualFold(m[1], "MostRecent") && m[2] == "1" && currentKey > 0 {
			mostRecentID = currentKey
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return mostRecentID, nil
}

// ParsePersonaName reads loginusers.vdf and returns the PersonaName for the given Steam ID.
func ParsePersonaName(steamID uint64) string {
	installPath, err := GetInstallPath()
	if err != nil {
		return ""
	}

	vdfPath := filepath.Join(installPath, "config", "loginusers.vdf")
	f, err := os.Open(vdfPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	targetID := strconv.FormatUint(steamID, 10)
	scanner := bufio.NewScanner(f)

	var inTarget bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect Steam ID section headers (bare quoted 17-digit number)
		if strings.HasPrefix(line, "\"") && !strings.Contains(line, "\t") {
			trimmed := strings.Trim(line, "\"")
			inTarget = trimmed == targetID
			continue
		}

		if !inTarget {
			continue
		}

		m := kvPairRe.FindStringSubmatch(line)
		if m != nil && strings.EqualFold(m[1], "PersonaName") {
			return m[2]
		}
	}

	return ""
}

// Close stops the watcher and releases resources.
func (w *AccountWatcher) Close() {
	close(w.stopCh)
	w.watcher.Close()
	w.wg.Wait()
	slog.Info("account watcher stopped")
}
