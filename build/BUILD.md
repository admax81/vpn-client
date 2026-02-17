# Building Installers

## Structure

```
build/
  windows/
    installer.iss    # Inno Setup script
    build.ps1        # PowerShell build helper
    icon.ico         # Generated icon (run gen-icon first)
  macos/
    build.ps1        # Cross-compile .app bundle from Windows → .zip
    create-dmg.sh    # .app bundle + .dmg (requires macOS)
    entitlements.plist
  gen-icon/
    main.go          # Icon generator
dist/                # Output directory (gitignored)
```

---

## Prerequisites

| What | Tools needed |
|------|-------------|
| Windows installer | Go 1.21+, [Inno Setup 6](https://jrsoftware.org/isdl.php) |
| macOS .zip / .dmg | Go 1.21+, **macOS** or GitHub Actions (CGo required) |
| macOS .dmg (on macOS) | Go 1.21+, Xcode CLI tools |

---

## Windows Installer (.exe)

### Quick (PowerShell)

```powershell
# Generate icon first
go run build/gen-icon/main.go

# Build installer (builds exe + runs Inno Setup)
.\build\windows\build.ps1 -Version "1.0.0"
```

Output: `dist/vpn-client-1.0.0-windows-setup.exe`

### Manual Steps

```powershell
# 1. Build the binary
$env:GOOS="windows"; $env:GOARCH="amd64"
go build -ldflags "-H=windowsgui -s -w" -o dist/windows-amd64/vpn-client.exe ./cmd/vpn-client

# 2. Generate icon (optional, for installer)
go run build/gen-icon/main.go

# 3. Run Inno Setup compiler
iscc build/windows/installer.iss
```

### What the installer does
- Installs to `C:\Program Files\VPN Client\`
- Creates Start Menu shortcuts
- Optional desktop shortcut
- Optional "Start on Windows startup" registry entry
- Kills running instance before install/uninstall
- Requires administrator privileges

---

## macOS — Сборка

> **Важно:** `fyne.io/systray` требует CGo (Objective-C / Cocoa) на macOS.
> Кросс-компиляция с Windows **невозможна** — нужен macOS или GitHub Actions.

### Вариант 1: GitHub Actions (рекомендуется)

```bash
git tag v1.0.0 && git push --tags
```

Или запустите workflow "Build macOS" вручную на GitHub.
См. `.github/workflows/build-macos.yml`.

### Вариант 2: На macOS напрямую

```bash
./build/macos/create-dmg.sh 1.0.0   # Собирает .app + .dmg
pwsh build/macos/build.ps1 -Version "1.0.0"  # Собирает .app + .zip
```

Output: `dist/vpn-client-1.0.0-macos.dmg` или `.zip` с `VPN Client.app`.

### Установка на macOS (для пользователя)
```bash
unzip vpn-client-1.0.0-macos.zip
chmod +x "VPN Client.app/Contents/MacOS/"*
mv "VPN Client.app" /Applications/
```

### Если нужен .dmg

`.dmg` можно создать только через `hdiutil` (macOS). Два варианта:

**Вариант A: GitHub Actions (рекомендуется)**
Скрипт CI ниже автоматически собирает .dmg на бесплатном macOS-раннере.

**Вариант B: На macOS напрямую**
```bash
chmod +x build/macos/create-dmg.sh
./build/macos/create-dmg.sh 1.0.0
```

### Code Signing (optional)

```bash
export CODESIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
./build/macos/create-dmg.sh 1.0.0

# Notarize
xcrun notarytool submit dist/vpn-client-1.0.0-macos.dmg \
    --apple-id "your@email.com" \
    --team-id "TEAMID" \
    --password "@keychain:AC_PASSWORD" \
    --wait
```

### macOS Icon

Place `AppIcon.icns` in `build/macos/` (requires macOS to generate):

```bash
mkdir AppIcon.iconset
sips -z 16 16     icon.png --out AppIcon.iconset/icon_16x16.png
sips -z 32 32     icon.png --out AppIcon.iconset/icon_16x16@2x.png
sips -z 32 32     icon.png --out AppIcon.iconset/icon_32x32.png
sips -z 64 64     icon.png --out AppIcon.iconset/icon_32x32@2x.png
sips -z 128 128   icon.png --out AppIcon.iconset/icon_128x128.png
sips -z 256 256   icon.png --out AppIcon.iconset/icon_128x128@2x.png
sips -z 256 256   icon.png --out AppIcon.iconset/icon_256x256.png
sips -z 512 512   icon.png --out AppIcon.iconset/icon_256x256@2x.png
sips -z 512 512   icon.png --out AppIcon.iconset/icon_512x512.png
sips -z 1024 1024 icon.png --out AppIcon.iconset/icon_512x512@2x.png
iconutil -c icns AppIcon.iconset
mv AppIcon.icns build/macos/
```

---

## Using Make (Linux/macOS)

```bash
make build-windows       # Cross-compile Windows exe
make build-macos         # Build macOS universal binary
make installer-windows   # Build Windows installer (requires iscc in PATH)
make installer-macos     # Build macOS .dmg (macOS only)
make all                 # Build all installers
make clean               # Remove dist/
```

---

## CI/CD — GitHub Actions (собирает .dmg без macOS у вас)

```yaml
name: Build Installers
on:
  push:
    tags: ['v*']

jobs:
  windows:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - name: Install Inno Setup
        run: choco install innosetup -y
      - name: Build installer
        run: .\build\windows\build.ps1 -Version "${{ github.ref_name }}"
      - uses: actions/upload-artifact@v4
        with:
          name: windows-installer
          path: dist/*-windows-setup.exe

  macos:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - name: Build DMG
        run: |
          chmod +x build/macos/create-dmg.sh
          ./build/macos/create-dmg.sh ${{ github.ref_name }}
      - uses: actions/upload-artifact@v4
        with:
          name: macos-dmg
          path: dist/*-macos.dmg
```
