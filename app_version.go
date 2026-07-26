package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// UpdateInfo contains the result of a version check against GitHub releases.
type UpdateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	DownloadURL     string `json:"downloadUrl"`
}

// GetAppVersion returns the current application version.
func (a *App) GetAppVersion() string {
	return Version
}

// CheckForUpdates checks GitHub for the latest release and compares with the current version.
func (a *App) CheckForUpdates() (UpdateInfo, error) {
	result := UpdateInfo{CurrentVersion: Version}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, "https://api.github.com/repos/steamforge-app/steamforge/releases/latest", nil)
	if err != nil {
		return result, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return result, fmt.Errorf("decode response: %w", err)
	}

	result.LatestVersion = release.TagName
	result.DownloadURL = release.HTMLURL
	result.UpdateAvailable = Version != "dev" && normalizeVersion(release.TagName) != normalizeVersion(Version)

	return result, nil
}

// normalizeVersion strips a leading "v" for comparison.
func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}
