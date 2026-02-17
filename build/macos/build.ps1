# Build macOS .app bundle — requires macOS (CGo + Cocoa)
#
# fyne.io/systray uses CGo with Objective-C on macOS, so cross-compilation
# from Windows with CGO_ENABLED=0 is NOT possible. The macOS-specific
# symbols (setInternalLoop, nativeLoop, registerSystray, etc.) are defined
# in systray_darwin.go which requires `import "C"`.
#
# How to build for macOS:
#   Option A (recommended): GitHub Actions — push a tag (v1.0.4) or run
#     the "Build macOS" workflow manually. See .github/workflows/build-macos.yml
#   Option B: On a Mac, run:  ./build/macos/create-dmg.sh 1.0.4
#   Option C: On a Mac, run this script via PowerShell for Mac (pwsh)
#
# Usage (macOS only): pwsh build/macos/build.ps1 -Version "1.0.4"

param(
    [string]$Version = "1.0.0"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Resolve-Path "$PSScriptRoot\..\.."
$Dist = Join-Path $ProjectRoot "dist"
$MacDir = Join-Path $Dist "macos"
$AppName = "VPN Client"
$BinaryName = "vpn-client"
$BundleId = "com.vpnclient.app"

# ─── Check: CGo cross-compilation is not possible from Windows ───
if ($IsWindows -or ($env:OS -eq "Windows_NT")) {
    Write-Host "=== ERROR: Cannot cross-compile macOS build from Windows ===" -ForegroundColor Red
    Write-Host ""
    Write-Host "  fyne.io/systray requires CGo (Objective-C / Cocoa) on macOS." -ForegroundColor Yellow
    Write-Host "  Cross-compiling CGo programs for macOS from Windows is not supported." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  Options:" -ForegroundColor Cyan
    Write-Host "    1. GitHub Actions (recommended):" -ForegroundColor White
    Write-Host "         git tag v$Version && git push --tags" -ForegroundColor DarkGray
    Write-Host "         Or run the 'Build macOS' workflow manually from GitHub." -ForegroundColor DarkGray
    Write-Host "    2. Build on macOS directly:" -ForegroundColor White
    Write-Host "         ./build/macos/create-dmg.sh $Version" -ForegroundColor DarkGray
    Write-Host "    3. Use this script on macOS with PowerShell:" -ForegroundColor White
    Write-Host "         pwsh build/macos/build.ps1 -Version `"$Version`"" -ForegroundColor DarkGray
    Write-Host ""
    throw "macOS build requires macOS or CI (CGo dependency: fyne.io/systray)"
}

Write-Host "=== VPN Client macOS Build ===" -ForegroundColor Cyan
Write-Host "Version: $Version"

# ─── Step 1: Build for macOS arm64 + amd64 (CGo enabled) ───
Write-Host "`n[1/3] Compiling Go binaries..." -ForegroundColor Yellow

New-Item -ItemType Directory -Force -Path $MacDir | Out-Null

Push-Location $ProjectRoot
try {
    foreach ($arch in @("arm64", "amd64")) {
        Write-Host "  Building darwin/$arch..."
        $env:GOOS = "darwin"
        $env:GOARCH = $arch
        $env:CGO_ENABLED = "1"

        go build -ldflags "-s -w -X main.Version=$Version" `
            -o "$MacDir/$BinaryName-$arch" ./cmd/vpn-client

        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed for darwin/$arch"
        }
    }
} finally {
    Pop-Location
    Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
}

Write-Host "  Both arm64 and amd64 binaries built" -ForegroundColor Green
Write-Host "  Note: 'lipo' (macOS-only) needed for universal binary." -ForegroundColor DarkGray
Write-Host "  The .zip will include both binaries; a launcher script picks the right one." -ForegroundColor DarkGray

# ─── Step 2: Create .app bundle structure ───
Write-Host "`n[2/3] Creating .app bundle..." -ForegroundColor Yellow

$AppDir = Join-Path $MacDir "$AppName.app"

# Clean previous build
if (Test-Path $AppDir) { Remove-Item -Recurse -Force $AppDir }

$ContentsDir = Join-Path $AppDir "Contents"
$MacOSDir = Join-Path $ContentsDir "MacOS"
$ResourcesDir = Join-Path $ContentsDir "Resources"

New-Item -ItemType Directory -Force -Path $MacOSDir | Out-Null
New-Item -ItemType Directory -Force -Path $ResourcesDir | Out-Null

# Copy both architecture binaries
Copy-Item "$MacDir\$BinaryName-arm64" "$MacOSDir\$BinaryName-arm64"
Copy-Item "$MacDir\$BinaryName-amd64" "$MacOSDir\$BinaryName-amd64"

# Create launcher script that picks the right binary based on arch
$launcher = @"
#!/bin/bash
DIR="`$(cd "`$(dirname "`$0")" && pwd)"
ARCH="`$(uname -m)"
if [ "`$ARCH" = "arm64" ]; then
    exec "`$DIR/$BinaryName-arm64" "`$@"
else
    exec "`$DIR/$BinaryName-amd64" "`$@"
fi
"@
# Write with LF line endings
$launcherPath = Join-Path $MacOSDir $BinaryName
[System.IO.File]::WriteAllText($launcherPath, $launcher.Replace("`r`n", "`n"))

# Info.plist
$infoPlist = @"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>$AppName</string>
    <key>CFBundleDisplayName</key>
    <string>$AppName</string>
    <key>CFBundleIdentifier</key>
    <string>$BundleId</string>
    <key>CFBundleVersion</key>
    <string>$Version</string>
    <key>CFBundleShortVersionString</key>
    <string>$Version</string>
    <key>CFBundleExecutable</key>
    <string>$BinaryName</string>
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
    <string>Copyright 2026 VPN Client</string>
</dict>
</plist>
"@
[System.IO.File]::WriteAllText((Join-Path $ContentsDir "Info.plist"), $infoPlist.Replace("`r`n", "`n"))

# Copy icon if exists
$icnsPath = Join-Path $PSScriptRoot "AppIcon.icns"
if (Test-Path $icnsPath) {
    Copy-Item $icnsPath "$ResourcesDir\AppIcon.icns"
}

# Copy example config
Copy-Item "$ProjectRoot\configs\config.example.yaml" "$ResourcesDir\"

Write-Host "  .app bundle created at: $AppDir" -ForegroundColor Green

# ─── Step 3: Create .zip ───
Write-Host "`n[3/3] Creating .zip archive..." -ForegroundColor Yellow

$ZipName = "vpn-client-$Version-macos.zip"
$ZipPath = Join-Path $Dist $ZipName

if (Test-Path $ZipPath) { Remove-Item $ZipPath }

# Compress from the macos directory so the .app is at the root of the zip
Compress-Archive -Path $AppDir -DestinationPath $ZipPath -CompressionLevel Optimal

# Clean up intermediate binaries
Remove-Item "$MacDir\$BinaryName-arm64" -ErrorAction SilentlyContinue
Remove-Item "$MacDir\$BinaryName-amd64" -ErrorAction SilentlyContinue

Write-Host "`n=== Build Complete ===" -ForegroundColor Green
Write-Host "App bundle: $AppDir"
Write-Host "ZIP:        $ZipPath"
Write-Host ""
Write-Host "Distribution options:" -ForegroundColor Yellow
Write-Host "  1. Distribute the .zip directly (users unzip + drag to /Applications)"
Write-Host "  2. Use GitHub Actions to convert to .dmg (see build/BUILD.md)"
Write-Host ""
Write-Host "Note: After unzipping on macOS, users may need to run:" -ForegroundColor DarkGray
Write-Host "  chmod +x '$AppName.app/Contents/MacOS/$BinaryName'" -ForegroundColor DarkGray
Write-Host "  chmod +x '$AppName.app/Contents/MacOS/$BinaryName-arm64'" -ForegroundColor DarkGray
Write-Host "  chmod +x '$AppName.app/Contents/MacOS/$BinaryName-amd64'" -ForegroundColor DarkGray
