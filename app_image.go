package main

import (
	"errors"
)

func (a *App) GetImageBase64(url string) (string, error) {
	a.mu.RLock()
	imageService := a.imageService
	a.mu.RUnlock()

	if imageService == nil {
		return "", errors.New("image service not available")
	}
	return imageService.GetImageBase64(url)
}
