#!/bin/bash
# VPN Client - macOS .app bundle and .dmg creator
# Usage: ./build/macos/create-dmg.sh [version]
#
# Prerequisites:
#   - Go toolchain (for cross-compilation or native build)
#   - macOS with hdiutil (for .dmg creation)
#   - Optional: codesign identity for signing

set -euo pipefail

VERSION="${1:-1.0.0}"
APP_NAME="VPN Client"
BUNDLE_ID="com.vpnclient.app"
BINARY_NAME="vpn-client"

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/dist/macos"
APP_DIR="${BUILD_DIR}/${APP_NAME}.app"
DMG_NAME="vpn-client-${VERSION}-macos"

# Signing identity (set to "-" for ad-hoc, or your Developer ID)
CODESIGN_IDENTITY="${CODESIGN_IDENTITY:--}"

echo "=== Building VPN Client for macOS ==="
echo "Version: ${VERSION}"
echo "Output:  ${BUILD_DIR}"

# ──────────────────────────────────────────────────────────────
# Step 1: Build the Go binary for macOS (arm64 + amd64 universal)
# ──────────────────────────────────────────────────────────────
mkdir -p "${BUILD_DIR}"

echo "Building arm64 binary..."
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
    -ldflags "-s -w -X main.Version=${VERSION}" \
    -o "${BUILD_DIR}/${BINARY_NAME}-arm64" \
    ./cmd/vpn-client

echo "Building amd64 binary..."
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build \
    -ldflags "-s -w -X main.Version=${VERSION}" \
    -o "${BUILD_DIR}/${BINARY_NAME}-amd64" \
    ./cmd/vpn-client

# Create universal binary if lipo is available
if command -v lipo &> /dev/null; then
    echo "Creating universal binary..."
    lipo -create \
        "${BUILD_DIR}/${BINARY_NAME}-arm64" \
        "${BUILD_DIR}/${BINARY_NAME}-amd64" \
        -output "${BUILD_DIR}/${BINARY_NAME}"
    rm "${BUILD_DIR}/${BINARY_NAME}-arm64" "${BUILD_DIR}/${BINARY_NAME}-amd64"
else
    echo "lipo not available, using arm64 binary only"
    mv "${BUILD_DIR}/${BINARY_NAME}-arm64" "${BUILD_DIR}/${BINARY_NAME}"
    rm -f "${BUILD_DIR}/${BINARY_NAME}-amd64"
fi

# ──────────────────────────────────────────────────────────────
# Step 2: Create .app bundle
# ──────────────────────────────────────────────────────────────
echo "Creating .app bundle..."

rm -rf "${APP_DIR}"
mkdir -p "${APP_DIR}/Contents/MacOS"
mkdir -p "${APP_DIR}/Contents/Resources"

# Copy binary
cp "${BUILD_DIR}/${BINARY_NAME}" "${APP_DIR}/Contents/MacOS/${BINARY_NAME}"
chmod +x "${APP_DIR}/Contents/MacOS/${BINARY_NAME}"

# Create Info.plist
cat > "${APP_DIR}/Contents/Info.plist" << PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleExecutable</key>
    <string>${BINARY_NAME}</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © 2026 VPN Client</string>
    <key>SMPrivilegedExecutables</key>
    <dict>
        <key>${BUNDLE_ID}.helper</key>
        <string>identifier &quot;${BUNDLE_ID}.helper&quot;</string>
    </dict>
</dict>
</plist>
PLIST

# Copy icon if exists, otherwise create a placeholder
if [ -f "${PROJECT_ROOT}/build/macos/AppIcon.icns" ]; then
    cp "${PROJECT_ROOT}/build/macos/AppIcon.icns" "${APP_DIR}/Contents/Resources/AppIcon.icns"
else
    echo "Warning: No AppIcon.icns found. See build/macos/README.md for icon creation."
fi

# Copy example config into Resources
cp "${PROJECT_ROOT}/configs/config.example.yaml" "${APP_DIR}/Contents/Resources/"

# ──────────────────────────────────────────────────────────────
# Step 3: Code sign (optional)
# ──────────────────────────────────────────────────────────────
if [ "${CODESIGN_IDENTITY}" != "skip" ]; then
    echo "Code signing with identity: ${CODESIGN_IDENTITY}"
    codesign --force --deep --sign "${CODESIGN_IDENTITY}" \
        --options runtime \
        --entitlements "${PROJECT_ROOT}/build/macos/entitlements.plist" \
        "${APP_DIR}" || echo "Warning: Code signing failed (continuing without signature)"
fi

# ──────────────────────────────────────────────────────────────
# Step 4: Create .dmg
# ──────────────────────────────────────────────────────────────
echo "Creating .dmg..."

DMG_TEMP="${BUILD_DIR}/dmg-temp"
DMG_PATH="${PROJECT_ROOT}/dist/${DMG_NAME}.dmg"

rm -rf "${DMG_TEMP}" "${DMG_PATH}"
mkdir -p "${DMG_TEMP}"

cp -r "${APP_DIR}" "${DMG_TEMP}/"

# Create symlink to /Applications for drag-and-drop install
ln -s /Applications "${DMG_TEMP}/Applications"

# Create .dmg
hdiutil create \
    -volname "${APP_NAME}" \
    -srcfolder "${DMG_TEMP}" \
    -ov \
    -format UDZO \
    -imagekey zlib-level=9 \
    "${DMG_PATH}"

rm -rf "${DMG_TEMP}"

echo ""
echo "=== Build complete ==="
echo "App bundle: ${APP_DIR}"
echo "DMG:        ${DMG_PATH}"
echo ""
echo "To install: Open the .dmg and drag VPN Client to Applications"
