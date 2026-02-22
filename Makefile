.PHONY: build build-windows build-linux dev clean frontend tag release

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

# Auto-bump patch version, create tag, and push (triggers GitHub release workflow)
# Override with: make tag TAG=v2.0.0
TAG ?= $(shell latest=$$(git tag --sort=-v:refname | head -1 | sed 's/^v//'); \
	major=$$(echo $$latest | cut -d. -f1); \
	minor=$$(echo $$latest | cut -d. -f2); \
	patch=$$(echo $$latest | cut -d. -f3); \
	echo "v$$major.$$minor.$$((patch + 1))")
tag:
	@echo "Tagging $(TAG)..."
	git tag $(TAG)
	git push origin $(TAG)

# Commit all changes and create a release tag
# Usage: make release m="your commit message"
# Override version: make release m="message" TAG=v2.0.0
release:
	git add -A
	git commit -m "$(m)"
	$(MAKE) tag
