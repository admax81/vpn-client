# VPN Client - Build & Packaging
# Usage:
#   make build-windows         Build Windows exe
#   make build-macos           Build macOS universal binary
#   make installer-windows     Build Windows Inno Setup installer
#   make installer-macos       Build macOS .dmg
#   make all                   Build all installers

VERSION   ?= 1.0.0
BINARY    := vpn-client
DIST      := dist
GOFLAGS   := -ldflags "-s -w -X main.Version=$(VERSION)"

# ──────────────────────────────────────────
# Binaries
# ──────────────────────────────────────────
.PHONY: build-windows build-macos build-linux

build-windows:
	@echo "=== Building Windows amd64 ==="
	@mkdir -p $(DIST)/windows-amd64
	# Embed manifest & icon via rsrc (assumes rsrc_windows.syso is up to date)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
		$(GOFLAGS) -ldflags "-H=windowsgui -s -w -X main.Version=$(VERSION)" \
		-o $(DIST)/windows-amd64/$(BINARY).exe ./cmd/vpn-client

build-macos:
	@echo "=== Building macOS (arm64 + amd64) ==="
	@echo "Note: fyne.io/systray requires CGO_ENABLED=1 on macOS (Cocoa/Obj-C)."
	@echo "      This target must be run on macOS, not cross-compiled from Windows."
	@mkdir -p $(DIST)/macos
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build \
		$(GOFLAGS) -o $(DIST)/macos/$(BINARY)-arm64 ./cmd/vpn-client
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build \
		$(GOFLAGS) -o $(DIST)/macos/$(BINARY)-amd64 ./cmd/vpn-client
	@if command -v lipo >/dev/null 2>&1; then \
		lipo -create $(DIST)/macos/$(BINARY)-arm64 $(DIST)/macos/$(BINARY)-amd64 \
			-output $(DIST)/macos/$(BINARY); \
		rm $(DIST)/macos/$(BINARY)-arm64 $(DIST)/macos/$(BINARY)-amd64; \
	else \
		mv $(DIST)/macos/$(BINARY)-arm64 $(DIST)/macos/$(BINARY); \
		rm -f $(DIST)/macos/$(BINARY)-amd64; \
	fi

build-linux:
	@echo "=== Building Linux amd64 ==="
	@mkdir -p $(DIST)/linux-amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		$(GOFLAGS) -o $(DIST)/linux-amd64/$(BINARY) ./cmd/vpn-client

# ──────────────────────────────────────────
# Installers
# ──────────────────────────────────────────
.PHONY: installer-windows installer-macos

installer-windows: build-windows
	@echo "=== Creating Windows Installer (Inno Setup) ==="
	iscc build/windows/installer.iss
	@echo "Installer: $(DIST)/vpn-client-$(VERSION)-windows-setup.exe"

installer-macos:
	@echo "=== Creating macOS DMG ==="
	chmod +x build/macos/create-dmg.sh
	./build/macos/create-dmg.sh $(VERSION)
	@echo "DMG: $(DIST)/vpn-client-$(VERSION)-macos.dmg"

# ──────────────────────────────────────────
# All
# ──────────────────────────────────────────
.PHONY: all clean

all: installer-windows installer-macos

clean:
	rm -rf $(DIST)
