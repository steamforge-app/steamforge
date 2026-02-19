package cache

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxImageSize = 5 * 1024 * 1024 // 5MB

// AllowedImageHosts is the canonical set of permitted Steam CDN hosts for image downloads.
var AllowedImageHosts = map[string]bool{
	"steamcdn-a.akamaihd.net":           true,
	"cdn.akamai.steamstatic.com":        true,
	"shared.cloudflare.steamstatic.com": true,
	"cdn.steamstatic.com":               true,
	"media.steampowered.com":            true,
	"avatars.steamstatic.com":           true,
	"store.cloudflare.steamstatic.com":  true,
	"shared.akamai.steamstatic.com":     true,
	"cdn.cloudflare.steamstatic.com":    true,
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if !AllowedImageHosts[req.URL.Hostname()] {
			return fmt.Errorf("redirect to disallowed host: %s", req.URL.Hostname())
		}
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

type ImageCache struct {
	dir    string
	mu     sync.RWMutex
	exists map[string]bool
}

func NewImageCache(dir string) (*ImageCache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &ImageCache{
		dir:    dir,
		exists: make(map[string]bool),
	}, nil
}

func (c *ImageCache) get(imageURL string) string {
	path := c.pathFor(imageURL)

	c.mu.RLock()
	if c.exists[imageURL] {
		c.mu.RUnlock()
		return path
	}
	c.mu.RUnlock()

	if _, err := os.Stat(path); err == nil {
		c.mu.Lock()
		c.exists[imageURL] = true
		c.mu.Unlock()
		return path
	}

	return ""
}

func (c *ImageCache) Download(imageURL string) (string, error) {
	if cached := c.get(imageURL); cached != "" {
		return cached, nil
	}

	slog.Debug("image cache miss, downloading", "url", imageURL)

	parsed, err := url.Parse(imageURL)
	if err != nil || !AllowedImageHosts[parsed.Hostname()] {
		return "", fmt.Errorf("disallowed image host: %s", imageURL)
	}

	resp, err := httpClient.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", imageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: status %d", imageURL, resp.StatusCode)
	}

	path := c.pathFor(imageURL)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	limited := io.LimitReader(resp.Body, maxImageSize+1)
	n, err := io.Copy(f, limited)
	if err != nil {
		os.Remove(path)
		return "", fmt.Errorf("write file: %w", err)
	}
	if n > maxImageSize {
		os.Remove(path)
		return "", fmt.Errorf("image exceeds %d byte size limit", maxImageSize)
	}

	c.mu.Lock()
	c.exists[imageURL] = true
	c.mu.Unlock()

	return path, nil
}

func (c *ImageCache) pathFor(imageURL string) string {
	hash := sha256.Sum256([]byte(imageURL))
	ext := filepath.Ext(imageURL)
	if ext == "" || len(ext) > 5 {
		ext = ".jpg"
	}
	name := fmt.Sprintf("%x%s", hash[:8], ext)
	return filepath.Join(c.dir, name)
}
