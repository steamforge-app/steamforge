.PHONY: build build-windows build-linux dev clean frontend

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Default: build for current platform
build:
	wails build -tags webkit2_41 -ldflags="-X main.Version=$(VERSION)"

# Windows .exe (run from Windows or WSL with wails in PATH)
build-windows:
	GOOS=windows wails build -ldflags="-s -w -X main.Version=$(VERSION)"

# Linux binary (Ubuntu 24.04 needs webkit2_41 tag)
build-linux:
	wails build -tags webkit2_41 -ldflags="-s -w -X main.Version=$(VERSION)"

# Development mode with hot reload
dev:
	wails dev -tags webkit2_41

# Build frontend only
frontend:
	cd frontend && npm run build

# Remove build output
clean:
	rm -rf build/bin/*
