# Variables
APP_NAME=mm-inactive-users
VERSION := $(shell cat VERSION)
EXISTING_TAG := $(shell git tag -l "$(VERSION)")

# Build for Linux
build-linux-amd64: pre-build-check
	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.Version=${VERSION}'" -o $(APP_NAME)_linux_amd64

build-linux-arm64: pre-build-check
	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 go build -ldflags="-X 'main.Version=${VERSION}'" -o $(APP_NAME)_linux_arm64

# Build for macOS
build-macos-apple: pre-build-check
	@echo "Building for macOS (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 go build -ldflags="-X 'main.Version=${VERSION}'" -o $(APP_NAME)_macos_apple

build-macos-intel: pre-build-check
	@echo "Building for macOS (intel)..."
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X 'main.Version=${VERSION}'" -o $(APP_NAME)_macos_intel

# Build for Windows
build-windows: pre-build-check
	@echo "Building for Windows..."
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X 'main.Version=${VERSION}'" -o $(APP_NAME)_windows.exe

build-all: build-linux-amd64 build-linux-arm64 build-macos-apple build-macos-intel build-windows

.PHONY: build-linux-amd64 build-linux-arm64 build-macos-apple build-macos-intel build-windows build-all

pre-build-check:
	@if [ "$(EXISTING_TAG)" = "$(VERSION)" ]; then \
		echo "Error: Tag $(VERSION) already exists.  Please update the VERSION file."; \
		exit 1; \
	fi

# Clean Up
clean:
	@echo "Cleaning up..."
	rm -f $(APP_NAME)_*
