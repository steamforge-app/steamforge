package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sync"

	"steamforge/internal/cache"
)

type ImageService struct {
	cache *cache.ImageCache
	ctx   context.Context
	mu    sync.Mutex
}

func NewImageService(cacheDir string) (*ImageService, error) {
	c, err := cache.NewImageCache(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("create image cache: %w", err)
	}
	return &ImageService{cache: c}, nil
}

func (s *ImageService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

func validateImageURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("scheme %q not allowed, must be https", u.Scheme)
	}
	if !cache.AllowedImageHosts[u.Hostname()] {
		return fmt.Errorf("host %q not allowed", u.Hostname())
	}
	return nil
}

func (s *ImageService) GetImageBase64(rawURL string) (string, error) {
	if rawURL == "" {
		return "", nil
	}

	if err := validateImageURL(rawURL); err != nil {
		slog.Warn("image URL rejected", "url", rawURL, "error", err)
		return "", fmt.Errorf("image URL not allowed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.cache.Download(rawURL)
	if err != nil {
		slog.Warn("image download failed", "url", rawURL, "error", err)
		return "", fmt.Errorf("download image: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read cached image: %w", err)
	}

	mimeType := "image/jpeg"
	if len(data) > 4 {
		if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
			mimeType = "image/png"
		} else if data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
			mimeType = "image/gif"
		}
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded), nil
}
